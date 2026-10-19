package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/vfasky/my-ai-audio/gui/internal/firmware"
	"github.com/vfasky/my-ai-audio/gui/internal/permission"
	"github.com/vfasky/my-ai-audio/gui/internal/refinement"
	"github.com/vfasky/my-ai-audio/gui/internal/vocabulary"
)

type intents struct {
	scan, cancel, warmup, install, configure int
	requested, settings                      []permission.Name
}

func testView(rec *intents) *View {
	return &View{
		Snapshot: Snapshot{Install: firmware.Initial(), Config: refinement.Default()},
		Actions: Actions{
			FirmwareScan: func() {}, FirmwareInstall: func() { rec.install++ }, FirmwareSelect: func(string) {},
			Scan: func() { rec.scan++ }, Forget: func() {}, Cancel: func() { rec.cancel++ }, Warmup: func() { rec.warmup++ },
			Select: func(string) {}, Put: func(string, vocabulary.Term) {}, Delete: func(string) {},
			Configure: func(refinement.Config) { rec.configure++ }, Credential: func(string, string) {}, DeleteCredential: func(string) {},
			RequestPermission: func(n permission.Name) { rec.requested = append(rec.requested, n) },
			OpenSettings:      func(n permission.Name) { rec.settings = append(rec.settings, n) },
		},
	}
}

func newTester(v *View) *ui.Tester {
	tt := ui.NewTester(v.Draw, 1120, 840)
	tt.SetDark(true)
	tt.SetTitleBar(ui.TitleBar{Height: TitleBarHeight, Left: 78})
	return tt
}

func TestDeviceActionsRespectSession(t *testing.T) {
	var rec intents
	v := testView(&rec)
	tt := newTester(v)
	if err := tt.Click("扫描 FoloToy AI Passport"); err != nil || rec.scan != 1 {
		t.Fatalf("scan: %v %d", err, rec.scan)
	}
	// Cancel is disabled while no speech session is active.
	_ = tt.Click("取消本次语音")
	if rec.cancel != 0 {
		t.Fatal("cancel fired without an active session")
	}
	v.Snapshot.SpeechActive = true
	tt.Frame()
	_ = tt.Click("准备 zh-CN 模型")
	if err := tt.Click("取消本次语音"); err != nil || rec.cancel != 1 || rec.warmup != 0 {
		t.Fatalf("active session gates: %v %+v", err, rec)
	}
}

func TestSidebarNavigatesAndInstallIsGated(t *testing.T) {
	var rec intents
	v := testView(&rec)
	tt := newTester(v)
	if err := tt.Click("固件安装"); err != nil || v.Page != 3 {
		t.Fatalf("navigate: %v page %d", err, v.Page)
	}
	_ = tt.Click("确认安装随附固件")
	if rec.install != 0 {
		t.Fatal("install fired before a device was selected")
	}
	if !tt.HasText("USB 设备") {
		t.Fatalf("firmware page texts %q", tt.Texts())
	}
}

func TestRefinementSegmentsUpdateDraft(t *testing.T) {
	var rec intents
	v := testView(&rec)
	v.Page = 2
	tt := newTester(v)
	if err := tt.Click("OpenAI 兼容接口"); err != nil || v.Config.Engine != "remote" {
		t.Fatalf("engine: %v %q", err, v.Config.Engine)
	}
	if !tt.HasText("POST 端点 + /chat/completions") {
		t.Fatalf("chat path missing: %q", tt.Texts())
	}
	if err := tt.Click("Responses"); err != nil || v.Config.API != refinement.APIResponses || !tt.HasText("POST 端点 + /responses") {
		t.Fatalf("api: %v %q", err, v.Config.API)
	}
	if err := tt.Click("结构化"); err != nil || v.Config.Mode != "structured" {
		t.Fatalf("mode: %v %q", err, v.Config.Mode)
	}
	if !tt.HasText("显式远端模式：最终转写和词库将发送到所填 HTTPS 端点；音频不发送。") {
		t.Fatal("remote disclosure missing")
	}
	if err := tt.Click("保存设置"); err != nil || rec.configure != 1 {
		t.Fatalf("save: %v", err)
	}
}

func TestWaveLevelsMirrorNewestAtCentre(t *testing.T) {
	var bins [32]float64
	bins[31] = 1
	bins[30] = 0.25
	l := waveLevels(bins)
	mid := waveBars / 2
	if l[mid] != 1 || l[mid-1] != l[mid+1] || l[mid-1] <= 0 || l[0] != 0 {
		t.Fatalf("levels %v", l)
	}
}

// TestSnapshots writes every page as PNG when GOVOICE_UI_SNAPSHOTS names a
// directory, for visual review against the design reference.
func TestSnapshots(t *testing.T) {
	dir := os.Getenv("GOVOICE_UI_SNAPSHOTS")
	if dir == "" {
		t.Skip("set GOVOICE_UI_SNAPSHOTS to write page snapshots")
	}
	var rec intents
	v := testView(&rec)
	s := &v.Snapshot
	s.Selected, s.Ready, s.Firmware, s.Phase, s.SpeechActive = "6F1C-22A0", true, "1.2.0", "recording", true
	s.Connection, s.Model, s.Status = "Connected; ATT payload ≥182 B", "ready", "Credential saved in Keychain"
	s.Devices = []Device{{ID: "6F1C-22A0", Name: "FoloToy AI Passport"}, {ID: "91AB-07CC", Name: "Go Voice"}}
	s.Partial, s.Original = "今天我们介绍自定义声音", "今天我们介绍自定义声音。只需几秒钟的音频即可克隆你的声音。"
	for i := range s.Meter.Bins {
		s.Meter.Bins[i] = []float64{0.01, 0.04, 0.12, 0.3, 0.08, 0.5, 0.2, 0.02}[i%8]
	}
	s.Meter.Bins[31], s.Meter.RMS, s.Meter.Peak, s.Meter.Frames = 0.62, 0.18, 0.62, 214
	s.Meter.Received = 214
	s.Terms = []vocabulary.Term{{Term: "Go Voice", NormalizedTerm: "go voice", Source: "manual", Weight: 50, Locked: true}, {Term: "SpeechAnalyzer", NormalizedTerm: "speechanalyzer", Source: "ai", Weight: 30}}
	if b, err := os.ReadFile("../../resources/icon.png"); err == nil {
		v.AppIcon, _ = ui.DecodeBitmap(b)
	}
	tt := newTester(v)
	tt.SetScale(2)
	write := func(name string) {
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	s.Permissions = permission.Set{permission.Bluetooth: permission.Granted, permission.Speech: permission.Undetermined, permission.Accessibility: permission.Denied, permission.Automation: permission.Unknown}
	for i := range pages {
		v.Page = i
		write(pages[i].id)
	}
	s.Permissions = permission.Set{permission.Bluetooth: permission.Granted, permission.Speech: permission.Granted, permission.Accessibility: permission.Granted, permission.Automation: permission.Granted}
	v.Page = 0
	for _, phase := range []string{"ready", "refining"} {
		s.Phase = phase
		write("voice-" + phase)
	}
}

func TestFirmwareErrorShowsCodeAndStage(t *testing.T) {
	var rec intents
	v := testView(&rec)
	v.Page = 3
	v.Snapshot.Install = firmware.State{Phase: "error", HelperReady: true, ErrorCode: "PREFLIGHT_READ_UNAVAILABLE", FailedStage: "preflight", Message: firmware.ErrorLabel("PREFLIGHT_READ_UNAVAILABLE")}
	tt := newTester(v)
	if !tt.HasText("错误码 PREFLIGHT_READ_UNAVAILABLE · 失败阶段：安装前检查") {
		t.Fatalf("texts %q", tt.Texts())
	}
}

func TestPermissionsOnboardingActions(t *testing.T) {
	var rec intents
	v := testView(&rec)
	v.Snapshot.Permissions = permission.Set{permission.Bluetooth: permission.Denied, permission.Speech: permission.Undetermined, permission.Accessibility: permission.Granted}
	tt := newTester(v)
	if !tt.HasText("开始使用前还需授权：蓝牙、语音识别。") {
		t.Fatalf("banner missing: %q", tt.Texts())
	}
	if err := tt.Click("前往授权"); err != nil || v.Page != PermissionsPage {
		t.Fatalf("banner navigation: %v page %d", err, v.Page)
	}
	if err := tt.Click("授权"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("打开系统设置"); err != nil {
		t.Fatal(err)
	}
	if len(rec.requested) == 0 || rec.requested[0] != permission.Speech || len(rec.settings) == 0 || rec.settings[0] != permission.Bluetooth {
		t.Fatalf("intents %+v", rec)
	}
	if !tt.HasText("必需权限已完成 1 / 3 项。授权后无需重启，状态会自动刷新。") {
		t.Fatalf("summary: %q", tt.Texts())
	}
}
