package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/egoist/mygo"
	nativeui "github.com/egoist/mygo/ui"
	"github.com/vfasky/my-ai-audio/gui/internal/device"
	"github.com/vfasky/my-ai-audio/gui/internal/firmware"
	"github.com/vfasky/my-ai-audio/gui/internal/permission"
	"github.com/vfasky/my-ai-audio/gui/internal/platform/bridge"
	"github.com/vfasky/my-ai-audio/gui/internal/platform/keychain"
	"github.com/vfasky/my-ai-audio/gui/internal/protocol"
	"github.com/vfasky/my-ai-audio/gui/internal/refinement"
	appui "github.com/vfasky/my-ai-audio/gui/internal/ui"
	"github.com/vfasky/my-ai-audio/gui/internal/vocabulary"
	"os"
	"path/filepath"
	"time"
)

type settings struct {
	Peripheral string            `json:"peripheral"`
	Refinement refinement.Config `json:"refinement"`
}
type application struct {
	ctx               context.Context
	stop              context.CancelFunc
	window            *mygo.Window
	view              appui.View
	coordinator       device.Coordinator
	installer         *firmware.Process
	installState      firmware.State
	installPaths      firmware.Paths
	bluetooth, speech *bridge.Process
	config            settings
	store             vocabulary.Store
	directory         string
	subscribed        bool
	hello             *protocol.Control
	quitting          bool
	refineCancel      context.CancelFunc
	localReply        chan string
	terms             []string
	permissions       permission.Set
	// onboarded is set once the first complete permission report decided
	// whether to open the permissions page.
	onboarded bool
	// The dictation capsule: a panel shown without the keyboard during a
	// session, lingering briefly after one ends without inserting.
	hud       *mygo.Window
	hudShown  bool
	hudUntil  time.Time
	escapeSet bool
	tray      *mygo.Tray
}

// Run starts the app; icon is the packaged PNG app icon shown in the sidebar
// and tray the menu-bar template icon.
func Run(icon, tray []byte) {
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &application{ctx: ctx, stop: cancel, config: settings{Refinement: refinement.Default()}, permissions: permission.Set{}}
	// The interface is designed dark only; keep the window chrome in step.
	mygo.Theme.SetSource(mygo.ThemeDark)
	// A menu-bar app: no Dock icon (LSUIElement in mygo.json from launch),
	// and closing the window keeps it running in the menu bar.
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.WhenReady(func() {
		if bitmap, err := nativeui.DecodeBitmap(icon); err == nil {
			a.view.AppIcon = bitmap
		}
		a.initialize()
		a.window = mygo.NewWindow(mygo.WindowOptions{
			Title: "go-voice", Width: 1120, Height: 840, MinWidth: 920, MinHeight: 660,
			TitleBarStyle: mygo.TitleBarHiddenInset, TitleBarHeight: appui.TitleBarHeight,
			BackgroundColor: "#0A0A0A",
			Content:         nativeui.View(a.view.Draw),
		})
		// Hide rather than close, except while quitting: a prevented close
		// would cancel the quit.
		a.window.OnClose(func(e *mygo.CloseEvent) {
			if a.quitting {
				return
			}
			e.PreventDefault()
			a.window.Hide()
		})
		mygo.App.Focus()
		a.startTray(tray)
		a.hud = mygo.NewWindow(mygo.WindowOptions{
			Title: "go-voice dictation", Panel: true, Hidden: true, Frameless: true, Transparent: true,
			DisableResize: true, Width: appui.HUDWidth, Height: appui.HUDHeight,
			Content: nativeui.View(a.view.DrawHUD),
		})
		a.hud.OnClose(func(e *mygo.CloseEvent) {
			if a.quitting {
				return
			}
			e.PreventDefault()
			a.effects(a.coordinator.Cancel("已取消"))
			a.hideHUD()
		})
		a.startNative()
	})
	mygo.App.OnBeforeQuit(func(_ *mygo.QuitEvent) {
		a.quitting = true
		a.effects(a.coordinator.Cancel("Application exiting"))
		if a.installer != nil {
			a.installer.Close()
			select {
			case <-a.installer.Done():
			case <-time.After(600 * time.Millisecond):
			}
		}
		a.bindEscape(false)
		cancel()
		if a.bluetooth != nil {
			a.bluetooth.Close()
		}
		if a.speech != nil {
			a.speech.Close()
		}
	})
	_ = mygo.App.Run()
}

// startTray puts go-voice in the menu bar: settings, a placeholder update
// check, and quit.
func (a *application) startTray(icon []byte) {
	tray, err := mygo.NewTray(mygo.TrayOptions{
		Icon:           icon,
		IconIsTemplate: true,
		ToolTip:        "go-voice",
		Menu: mygo.NewMenu([]*mygo.MenuItem{
			{Label: "设置…", Accelerator: "CmdOrCtrl+,", Click: func(*mygo.MenuItem, *mygo.Window) { a.showMain() }},
			{Label: "检查更新…", Click: func(*mygo.MenuItem, *mygo.Window) { a.checkForUpdates() }},
			mygo.Separator(),
			{Label: "退出 go-voice", Accelerator: "CmdOrCtrl+Q", Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
		}),
	})
	if err != nil {
		a.view.Snapshot.Status = "菜单栏图标不可用"
		return
	}
	a.tray = tray
}
func (a *application) showMain() {
	if a.window == nil {
		return
	}
	a.window.Show()
	mygo.App.Focus()
}

// checkForUpdates is a placeholder until an update feed exists.
func (a *application) checkForUpdates() {
	version := mygo.App.Version()
	if version == "" {
		version = "开发版"
	}
	mygo.App.Focus()
	// Message is modal: keep it off the main thread.
	go func() {
		_, _ = mygo.Dialog.Message(mygo.MessageOptions{
			Type:    mygo.MessageInfo,
			Title:   "检查更新",
			Message: "检查更新功能即将推出",
			Detail:  "当前版本 " + version + "。",
		})
	}()
}
func (a *application) post(fn func()) {
	mygo.RunOnMain(func() {
		if a.quitting {
			return
		}
		fn()
		a.refresh()
	})
}
func (a *application) initialize() {
	a.installState = firmware.Initial()
	base, e := os.UserConfigDir()
	if e != nil {
		a.view.Snapshot.Status = "Application support unavailable"
		base = ""
	}
	a.directory = filepath.Join(base, "MyGo-GoVoice")
	a.store, e = vocabulary.Load(filepath.Join(a.directory, "vocabulary.json"))
	if e != nil {
		a.view.Snapshot.Status = "Vocabulary load failed"
	}
	if b, e := os.ReadFile(filepath.Join(a.directory, "settings.json")); e == nil && len(b) < 16384 {
		var s settings
		if json.Unmarshal(b, &s) == nil {
			a.config = s
		}
	}
	a.view.Actions = appui.Actions{
		FirmwareScan: func() {
			if a.installState.Scan() {
				a.sendInstaller(firmware.Request{Type: "scan"})
				a.refresh()
			}
		},
		FirmwareSelect:  func(id string) { a.installState.Select(id); a.refresh() },
		FirmwareInstall: a.installFirmware,
		Scan:            func() { a.sendBluetooth(map[string]any{"type": "scan"}) },
		Select: func(id string) {
			if a.installState.Busy() {
				return
			}
			a.effects(a.coordinator.Disconnect())
			a.subscribed = false
			a.hello = nil
			a.config.Peripheral = id
			a.saveSettings()
			a.sendBluetooth(map[string]any{"type": "set-preferred-device", "deviceId": id})
		},
		Forget: func() {
			a.effects(a.coordinator.Disconnect())
			a.config.Peripheral = ""
			a.subscribed = false
			a.hello = nil
			a.saveSettings()
			a.sendBluetooth(map[string]any{"type": "set-preferred-device", "deviceId": nil})
		},
		Cancel: func() { a.effects(a.coordinator.Cancel("已取消")) },
		Commit: func() {
			// The panel took the keyboard when clicked: hide it first so the
			// paste reaches the app the user dictated into.
			a.hideHUD()
			if a.refineCancel != nil {
				a.refineCancel()
				a.refineCancel = nil
			}
			a.localReply = nil
			a.effects(a.coordinator.Commit())
		},
		Warmup: func() {
			if !a.coordinator.Active {
				a.sendSpeech(map[string]any{"type": "warmup"})
			}
		},
		RequestPermission: a.requestPermission,
		OpenSettings: func(n permission.Name) {
			if e := mygo.Shell.OpenExternal(permission.SettingsURL(n)); e != nil {
				a.view.Snapshot.Status = "无法打开系统设置"
			}
		},
		Put: func(previous string, t vocabulary.Term) {
			if e := a.store.Put(t, false); e != nil {
				a.view.Snapshot.Status = e.Error()
			} else {
				if previous != "" && vocabulary.Normalize(previous) != vocabulary.Normalize(t.Term) {
					_ = a.store.Delete(previous, false)
				}
				a.saveVocabulary()
			}
			a.refresh()
		},
		Delete:    func(term string) { _ = a.store.Delete(term, false); a.saveVocabulary(); a.refresh() },
		Configure: func(c refinement.Config) { a.config.Refinement = c; a.saveSettings(); a.refresh() },
		Credential: func(endpoint, secret string) {
			go func() {
				e := (keychain.Store{}).Set(ctxOrBackground(a.ctx), endpoint, secret)
				a.post(func() {
					if e != nil {
						a.view.Snapshot.Status = "Keychain save failed"
					} else {
						a.view.Snapshot.Status = "Credential saved in Keychain"
					}
				})
			}()
		},
		DeleteCredential: func(endpoint string) {
			go func() {
				e := (keychain.Store{}).Delete(a.ctx, endpoint)
				a.post(func() {
					if e != nil {
						a.view.Snapshot.Status = "Keychain delete failed"
					} else {
						a.view.Snapshot.Status = "Credential deleted"
					}
				})
			}()
		},
	}
	a.refresh()
}
func ctxOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
func (a *application) saveSettings() {
	if e := os.MkdirAll(a.directory, 0700); e != nil {
		a.view.Snapshot.Status = "Settings unavailable"
		return
	}
	b, _ := json.Marshal(a.config)
	tmp := filepath.Join(a.directory, "settings.json.tmp")
	if e := os.WriteFile(tmp, b, 0600); e == nil {
		e = os.Rename(tmp, filepath.Join(a.directory, "settings.json"))
		if e != nil {
			a.view.Snapshot.Status = "Settings save failed"
		}
	} else {
		a.view.Snapshot.Status = "Settings save failed"
	}
}
func (a *application) saveVocabulary() {
	if e := a.store.Save(filepath.Join(a.directory, "vocabulary.json")); e != nil {
		a.view.Snapshot.Status = "Vocabulary save failed"
	}
}
func (a *application) refresh() {
	s := &a.view.Snapshot
	s.Install = a.installState
	s.SpeechActive = a.coordinator.Active
	c := &a.coordinator
	s.Selected = a.config.Peripheral
	s.Ready = c.Ready
	s.Phase = c.Phase
	s.Meter = c.Meter
	s.Partial = c.Partial
	s.Original = c.Original
	s.Refined = c.Refined
	s.Terms = append([]vocabulary.Term(nil), a.store.Terms...)
	s.Config = a.config.Refinement
	s.CanCommit = c.CanCommit()
	s.Permissions = make(permission.Set, len(a.permissions))
	for n, st := range a.permissions {
		s.Permissions[n] = st
	}
	if c.Status != "" {
		s.Status = c.Status
	}
	if a.window != nil {
		a.window.Invalidate()
	}
	a.syncHUD()
}

const hudLinger = 1500 * time.Millisecond

// syncHUD shows the capsule while a session records and processes, hides it
// before insertion, and keeps a cancelled or failed session's reason briefly.
func (a *application) syncHUD() {
	if a.hud == nil {
		return
	}
	c := &a.coordinator
	switch {
	case c.Active && c.Phase != "inserting":
		a.hudUntil = time.Time{}
		if !a.hudShown {
			d := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
			a.hud.SetPosition(d.WorkArea.X+(d.WorkArea.Width-appui.HUDWidth)/2, d.WorkArea.Y+d.WorkArea.Height-appui.HUDHeight-8)
			a.hud.ShowInactive()
			a.hudShown = true
		}
		a.bindEscape(true)
	case a.hudShown && c.Active:
		a.hideHUD()
	case a.hudShown:
		a.bindEscape(false)
		if a.hudUntil.IsZero() {
			a.hudUntil = time.Now().Add(hudLinger)
			time.AfterFunc(hudLinger+50*time.Millisecond, func() { a.post(func() {}) })
		} else if time.Now().After(a.hudUntil) {
			a.hideHUD()
		}
	}
	if a.hudShown {
		a.hud.Invalidate()
	}
}
func (a *application) hideHUD() {
	a.bindEscape(false)
	a.hudUntil = time.Time{}
	if a.hud != nil && a.hudShown {
		a.hud.Hide()
	}
	a.hudShown = false
}

// bindEscape claims Escape system-wide only while a session runs, so the
// capsule cancels it without taking the keyboard from the target app.
func (a *application) bindEscape(on bool) {
	switch {
	case on && !a.escapeSet:
		a.escapeSet = mygo.GlobalShortcut.Register("Escape", func() {
			a.post(func() { a.effects(a.coordinator.Cancel("已取消")) })
		}) == nil
	case !on && a.escapeSet:
		mygo.GlobalShortcut.Unregister("Escape")
		a.escapeSet = false
	}
}
func nativePath(name string) string {
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(exe), "..", "Resources", "native", name+".app", "Contents", "MacOS", name)
}
func (a *application) startNative() {
	a.startInstaller()
	var e error
	a.speech, e = bridge.Start(a.ctx, nativePath("SpeechBridge"), func(event bridge.Event) { a.post(func() { a.onSpeech(event) }) })
	if e != nil {
		a.view.Snapshot.Status = "SpeechBridge failed to launch"
		return
	}
	a.bluetooth, e = bridge.Start(a.ctx, nativePath("BluetoothHelper"), func(event bridge.Event) { a.post(func() { a.onBluetooth(event) }) })
	if e != nil {
		a.view.Snapshot.Status = "BluetoothHelper failed to launch"
		return
	}
	if a.config.Peripheral != "" {
		a.sendBluetooth(map[string]any{"type": "set-preferred-device", "deviceId": a.config.Peripheral})
	}
	a.sendSpeech(map[string]any{"type": "warmup"})
	a.queryPermissions()
	go func() {
		// Grants change in System Settings without telling the app: re-read
		// them while any is missing, so the onboarding updates on its own.
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				a.post(func() {
					if len(a.permissions.Missing()) > 0 || a.permissions[permission.Automation] != permission.Granted {
						a.queryPermissions()
					}
				})
			}
		}
	}()
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case now := <-ticker.C:
				a.post(func() { a.effects(a.coordinator.Tick(now)) })
			}
		}
	}()
}
func (a *application) startInstaller() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	a.installPaths = firmware.BundlePaths(exe)
	p, err := firmware.Start(a.ctx, a.installPaths.Helper)
	if err != nil {
		return
	}
	a.installer = p
	// At most one installer update may be queued on the native main thread.
	pending := make(chan struct{}, 1)
	started := time.Now()
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				select {
				case pending <- struct{}{}:
				default:
					continue
				}
				a.post(func() {
					defer func() { <-pending }()
					if a.installer != p {
						return
					}
					select {
					case <-p.Done():
						a.installState.Unavailable()
						a.installer = nil
						return
					default:
					}
					for i := 0; i < 64; i++ {
						select {
						case event := <-p.Events():
							if a.installState.Apply(event) {
								a.view.Snapshot.Devices = nil
								a.sendBluetooth(map[string]any{"type": "scan"})
							}
						default:
							if !a.installState.HelperReady && a.installState.Phase == "unavailable" && time.Since(started) > 10*time.Second {
								p.Close()
								a.installer = nil
							}
							return
						}
					}
				})
			}
		}
	}()
}
func (a *application) sendInstaller(r firmware.Request) {
	if a.installer == nil {
		a.installState.Unavailable()
		return
	}
	if err := a.installer.Send(r); err != nil {
		a.installer.Close()
		a.installer = nil
		a.installState.Unavailable()
	}
}
func (a *application) installFirmware() {
	if !a.installState.Install(a.coordinator.Active) {
		return
	}
	// Forget the BLE preference before installing so restart cannot silently bind it.
	a.effects(a.coordinator.Disconnect())
	a.subscribed = false
	a.hello = nil
	a.config.Peripheral = ""
	a.saveSettings()
	a.view.Snapshot.Connection = "已断开，正在安装固件"
	a.sendBluetooth(map[string]any{"type": "set-preferred-device", "deviceId": nil})
	a.sendInstaller(firmware.Request{Type: "install", Port: a.installState.Selected, FirmwarePath: a.installPaths.Firmware, ManifestPath: a.installPaths.Manifest})
	a.refresh()
}
func (a *application) sendBluetooth(v any) {
	if a.bluetooth != nil {
		if e := a.bluetooth.Send(v); e != nil {
			a.bluetooth.Close()
			a.bluetooth = nil
			a.effects(a.coordinator.Disconnect())
			a.view.Snapshot.Status = "Bluetooth pipe unavailable"
		}
	}
}
func (a *application) sendSpeech(v any) {
	if a.speech != nil {
		if e := a.speech.Send(v); e != nil {
			a.speech.Close()
			a.speech = nil
			a.effects(a.coordinator.Cancel("Speech pipe unavailable"))
			a.view.Snapshot.Status = "Speech pipe unavailable"
		}
	}
}
func (a *application) queryPermissions() {
	a.sendBluetooth(map[string]any{"type": "permissions"})
	a.sendSpeech(map[string]any{"type": "permissions"})
}

// requestPermission shows the system request while one can still appear,
// otherwise System Settings. Bluetooth asks on its own when the helper starts.
func (a *application) requestPermission(n permission.Name) {
	if n == permission.Bluetooth || !permission.Prompts(a.permissions[n]) {
		a.view.Actions.OpenSettings(n)
		return
	}
	a.sendSpeech(map[string]any{"type": "request-permission", "text": string(n)})
}

// onPermission records a helper report. The first time every grant has been
// reported, a missing required one opens the permissions page; a session that
// started without Speech ends with a clear reason rather than a timeout.
func (a *application) onPermission(e bridge.Event) {
	n, st, ok := permission.Parse(e.Permission, e.State)
	if !ok {
		return
	}
	a.permissions[n] = st
	if n == permission.Speech && st != permission.Granted && a.coordinator.Active {
		a.effects(a.coordinator.Cancel("语音识别未授权：请到“权限”页授权后重试"))
	}
	if a.onboarded {
		return
	}
	for _, name := range []permission.Name{permission.Bluetooth, permission.Speech, permission.Accessibility} {
		if _, reported := a.permissions[name]; !reported {
			return
		}
	}
	a.onboarded = true
	if len(a.permissions.Missing()) > 0 && !a.installState.Busy() {
		a.view.Page = appui.PermissionsPage
	}
}

func (a *application) onBluetooth(e bridge.Event) {
	if e.Type == "permission" {
		a.onPermission(e)
		return
	}
	if a.installState.Busy() {
		return
	}
	switch e.Type {
	case "discovered":
		for _, d := range a.view.Snapshot.Devices {
			if d.ID == e.DeviceID {
				return
			}
		}
		if len(a.view.Snapshot.Devices) < 64 {
			a.view.Snapshot.Devices = append(a.view.Snapshot.Devices, appui.Device{ID: e.DeviceID, Name: e.DeviceName})
		}
	case "connected":
		a.view.Snapshot.Connection = "Connected; ATT payload ≥182 B"
	case "subscribed":
		a.subscribed = true
		if a.hello != nil {
			a.effects(a.coordinator.Control(*a.hello, time.Now()))
		}
	case "control":
		v, err := protocol.ParseControl(e.Payload)
		if err != nil {
			a.effects(a.coordinator.Cancel("Invalid BLE control"))
			return
		}
		if v.Type == "hello" {
			a.hello = &v
			a.view.Snapshot.Firmware = v.Firmware
			if !a.subscribed {
				return
			}
		}
		a.effects(a.coordinator.Control(v, time.Now()))
	case "audio":
		a.effects(a.coordinator.Audio(protocol.Frame{SessionID: e.SessionID, Sequence: e.Sequence, Payload: e.Payload}, time.Now()))
	case "error", "disconnected":
		a.subscribed = false
		a.hello = nil
		a.view.Snapshot.Connection = "Disconnected"
		a.effects(a.coordinator.Disconnect())
	}
}
func (a *application) onSpeech(e bridge.Event) {
	if e.Type == "permission" {
		a.onPermission(e)
		return
	}
	if e.Type == "model" {
		a.view.Snapshot.Model = e.Message
		return
	}
	if e.Type == "disconnected" {
		a.effects(a.coordinator.Cancel("Speech bridge exited"))
		return
	}
	if e.Type == "refined" && e.SessionID == a.coordinator.Generation && a.localReply != nil {
		select {
		case a.localReply <- e.Text:
		default:
		}
		return
	}
	a.effects(a.coordinator.Speech(e.Type, e.SessionID, e.Text, time.Now()))
}
func (a *application) effects(effects []device.Effect) {
	for _, e := range effects {
		switch e.Type {
		case "host_ready":
			a.sendBluetooth(map[string]any{"type": "host_ready"})
		case "host_state":
			a.sendBluetooth(map[string]any{"type": "host_state", "state": e.State})
		case "start":
			a.terms = a.store.Top(100)
			a.saveVocabulary()
			a.sendSpeech(map[string]any{"type": "start", "sessionId": e.ID, "terms": a.terms})
		case "pcm":
			a.sendSpeech(map[string]any{"type": "pcm", "sessionId": e.ID, "sequence": e.Sequence, "payloadBase64": base64.StdEncoding.EncodeToString(e.PCM)})
		case "cancel":
			if a.refineCancel != nil {
				a.refineCancel()
				a.refineCancel = nil
			}
			a.localReply = nil
			a.sendSpeech(map[string]any{"type": "cancel", "sessionId": e.ID})
		case "refine":
			a.refine(e)
		case "insert":
			// Never paste while the capsule could hold the keyboard.
			a.hideHUD()
			a.sendSpeech(map[string]any{"type": "insert", "sessionId": e.ID, "text": e.Text})
		default:
			a.sendSpeech(map[string]any{"type": e.Type, "sessionId": e.ID, "text": e.Text})
		}
	}
	a.refresh()
}
func (a *application) refine(e device.Effect) {
	ctx, cancel := context.WithCancel(a.ctx)
	a.refineCancel = cancel
	reply := make(chan string, 1)
	a.localReply = reply
	cfg := a.config.Refinement
	terms := append([]string(nil), a.terms...)
	speech := a.speech
	go func() {
		out, status := refinement.Run(ctx, cfg, e.Text, terms, keychain.Store{}, func(ctx context.Context, prompt string) (string, error) {
			if speech == nil {
				return "", errors.New("speech unavailable")
			}
			if err := speech.Send(map[string]any{"type": "refine", "sessionId": e.ID, "text": prompt}); err != nil {
				return "", err
			}
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case result := <-reply:
				return result, nil
			}
		})
		a.post(func() {
			if ctx.Err() != nil {
				return
			}
			a.view.Snapshot.Status = status
			a.effects(a.coordinator.Speech("refinement_done", e.ID, out, time.Now()))
		})
	}()
}
