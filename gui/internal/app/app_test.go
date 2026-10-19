package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vfasky/my-ai-audio/gui/internal/firmware"
	"github.com/vfasky/my-ai-audio/gui/internal/permission"
	"github.com/vfasky/my-ai-audio/gui/internal/platform/bridge"
	"github.com/vfasky/my-ai-audio/gui/internal/refinement"
	appui "github.com/vfasky/my-ai-audio/gui/internal/ui"
	"github.com/vfasky/my-ai-audio/gui/internal/vocabulary"
)

func installReady() firmware.State {
	s := firmware.Initial()
	s.Apply(firmware.Event{Type: "ready"})
	s.Scan()
	s.Apply(firmware.Event{Type: "ports", Devices: []firmware.Device{{ID: "/dev/cu.usbmodem-secret", Name: "serial-secret"}}})
	s.Select("/dev/cu.usbmodem-secret")
	return s
}
func TestInstallRefusesActiveSpeech(t *testing.T) {
	a := &application{installState: installReady()}
	a.coordinator.Active = true
	a.config.Peripheral = "existing-ble"
	a.installFirmware()
	if a.installState.Phase != "ready" || !a.coordinator.Active || a.config.Peripheral != "existing-ble" {
		t.Fatal("active speech changed")
	}
}
func TestInstallDisconnectsAndDoesNotPersistUSB(t *testing.T) {
	a := &application{installState: installReady(), directory: t.TempDir()}
	a.config.Peripheral = "old-ble"
	a.coordinator.Ready = true
	a.subscribed = true
	a.installFirmware()
	if a.config.Peripheral != "" || a.coordinator.Ready || a.subscribed {
		t.Fatal("BLE selection retained")
	}
	if a.installState.Phase != "unavailable" {
		t.Fatal("missing helper not isolated")
	}
	b, err := os.ReadFile(filepath.Join(a.directory, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "secret") || strings.Contains(string(b), "old-ble") {
		t.Fatal("device identifier persisted")
	}
}
func TestInstallBlocksBLEEvents(t *testing.T) {
	a := &application{installState: installReady()}
	if !a.installState.Install(false) {
		t.Fatal("start")
	}
	a.onBluetooth(bridge.Event{Type: "subscribed"})
	a.onBluetooth(bridge.Event{Type: "connected"})
	if a.subscribed || a.coordinator.Ready || a.view.Snapshot.Connection != "" {
		t.Fatal("BLE events accepted during install")
	}
	// App refresh snapshots expose the same gate as the reducer.
	a.refresh()
	if !a.view.Snapshot.Install.Busy() || a.view.Snapshot.Install.CanInstall(false) {
		t.Fatal("incorrect UI gate")
	}
}

func perm(name, state string) bridge.Event {
	return bridge.Event{Type: "permission", Permission: name, State: state}
}
func TestFirstCompletePermissionReportOpensOnboarding(t *testing.T) {
	a := &application{permissions: permission.Set{}, installState: firmware.Initial()}
	a.onBluetooth(perm("bluetooth", "granted"))
	a.onSpeech(perm("speech", "undetermined"))
	if a.view.Page != 0 || a.onboarded {
		t.Fatal("decided before every required grant reported")
	}
	a.onSpeech(perm("accessibility", "granted"))
	if a.view.Page != appui.PermissionsPage || !a.onboarded {
		t.Fatalf("page %d", a.view.Page)
	}
	a.view.Page = 0
	a.onSpeech(perm("speech", "denied"))
	if a.view.Page != 0 {
		t.Fatal("onboarding must only open once")
	}
}
func TestSpeechWithoutGrantEndsSessionWithReason(t *testing.T) {
	a := &application{permissions: permission.Set{}, installState: firmware.Initial(), onboarded: true}
	a.coordinator.Active = true
	a.onSpeech(perm("speech", "undetermined"))
	if a.coordinator.Active || !strings.Contains(a.coordinator.Status, "语音识别未授权") {
		t.Fatalf("status %q", a.coordinator.Status)
	}
}

func TestUpdatesUnavailableDetail(t *testing.T) {
	for _, development := range []bool{true, false} {
		if got := updatesUnavailableDetail("", development); !strings.Contains(got, "开发版") {
			t.Fatalf("unversioned build: %q", got)
		}
	}
	if got := updatesUnavailableDetail("0.1.0", true); !strings.Contains(got, "开发版") {
		t.Fatalf("debug build: %q", got)
	}
	got := updatesUnavailableDetail("0.1.0", false)
	if !strings.Contains(got, "0.1.0") || !strings.Contains(got, "~/Applications") || strings.Contains(got, "开发版") {
		t.Fatalf("release build: %q", got)
	}
}

func TestRemoteTestMessage(t *testing.T) {
	for _, c := range []struct {
		err  error
		want string
	}{
		{refinement.ErrConfig, "https://"},
		{refinement.ErrKey, "API key"},
		{fmt.Errorf("post: %w", context.DeadlineExceeded), "超时"},
		{refinement.ErrResponse, "接口类型"},
		{&refinement.StatusError{Code: 401}, "HTTP 401"},
		{&refinement.StatusError{Code: 403}, "没有权限"},
		{&refinement.StatusError{Code: 404}, "/v1"},
		{&refinement.StatusError{Code: 429}, "额度"},
		{&refinement.StatusError{Code: 503}, "HTTP 503"},
		{&refinement.StatusError{Code: 400}, "HTTP 400"},
		{errors.New("dial tcp: no such host"), "无法连接"},
	} {
		if got, ok := remoteTestMessage(refinement.TestResult{}, c.err); ok || !strings.Contains(got, c.want) {
			t.Errorf("%v: %q %v", c.err, got, ok)
		}
	}
	long := strings.Repeat("好", 80)
	got, ok := remoteTestMessage(refinement.TestResult{Text: long, ThinkingOff: true, Elapsed: 1200 * time.Millisecond}, nil)
	if !ok || !strings.HasPrefix(got, "连接成功（1.2 秒，已请求关闭思考模式）") || !strings.HasSuffix(got, "…") || strings.Count(got, "好") != 60 {
		t.Fatalf("success: %q %v", got, ok)
	}
	if got, _ := remoteTestMessage(refinement.TestResult{Text: "好"}, nil); !strings.Contains(got, "不接受关闭思考") {
		t.Fatalf("thinking default: %q", got)
	}
}

func TestSettingsMessage(t *testing.T) {
	remote := refinement.Config{Engine: "remote", Mode: "light", Endpoint: "https://provider.example/v1", Model: "m"}
	for _, c := range []struct {
		name  string
		cfg   refinement.Config
		saved error
		typed bool
		key   error
		want  string
		ok    bool
	}{
		{"local", refinement.Default(), nil, false, nil, "设置已保存。", true},
		{"remote with key", remote, nil, true, nil, "设置已保存，API key 已存入钥匙串。", true},
		{"key failed", remote, nil, true, errors.New("denied"), "存入钥匙串失败", false},
		{"file failed", remote, errors.New("disk"), false, nil, "保存设置失败", false},
		{"remote incomplete", refinement.Config{Engine: "remote", Endpoint: "http://x"}, nil, false, nil, "远端润色还需要", false},
	} {
		got, ok := settingsMessage(c.cfg, c.saved, c.typed, c.key)
		if ok != c.ok || !strings.Contains(got, c.want) {
			t.Errorf("%s: %q %v", c.name, got, ok)
		}
	}
}

func TestCredentialPrecheck(t *testing.T) {
	if msg, ok := credentialPrecheck(" ", "k"); ok || !strings.Contains(msg, "端点") {
		t.Fatalf("no endpoint: %q", msg)
	}
	if msg, ok := credentialPrecheck("https://provider.example/v1", ""); ok || !strings.Contains(msg, "API key") {
		t.Fatalf("no key: %q", msg)
	}
	if _, ok := credentialPrecheck("https://provider.example/v1", "k"); !ok {
		t.Fatal("valid credential refused")
	}
}

func TestSaveSettingsWritesFile(t *testing.T) {
	a := &application{directory: filepath.Join(t.TempDir(), "settings"), config: settings{Refinement: refinement.Default()}}
	a.config.Refinement.Model = "m"
	if err := a.writeSettings(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(a.directory, "settings.json"))
	if err != nil || !strings.Contains(string(b), `"model":"m"`) {
		t.Fatalf("settings %s %v", b, err)
	}
	if info, _ := os.Stat(filepath.Join(a.directory, "settings.json")); info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", info.Mode())
	}
}

func TestSpeechAppliesVocabularyAliases(t *testing.T) {
	a := &application{}
	_ = a.store.PutReplacement("", vocabulary.Replacement{From: "施玉宝", To: "私域宝"})
	a.coordinator.Active, a.coordinator.Generation, a.coordinator.Phase = true, 7, "recording"
	a.onSpeech(bridge.Event{Type: "partial", SessionID: 7, Text: "打开施玉宝"})
	if a.coordinator.Partial != "打开私域宝" {
		t.Fatalf("partial %q", a.coordinator.Partial)
	}
	a.refresh()
	if len(a.view.Snapshot.Replacements) != 1 {
		t.Fatalf("snapshot %+v", a.view.Snapshot.Replacements)
	}
}

func TestVocabularyErrorMessages(t *testing.T) {
	for e, want := range map[error]string{
		vocabulary.ErrReplacementTaken:   "已经有替换词",
		vocabulary.ErrReplacementIsTerm:  "不能被替换",
		vocabulary.ErrInvalidReplacement: "64",
		errors.New("invalid term"):       "未保存",
	} {
		if got := vocabularyError(e); !strings.Contains(got, want) {
			t.Errorf("%v: %q", e, got)
		}
	}
}

func TestRefinementOffInsertsReplacedText(t *testing.T) {
	a := &application{config: settings{Refinement: refinement.Config{Engine: "remote", Mode: "off"}}}
	for _, r := range []vocabulary.Replacement{{From: "石玉宝", To: "私域宝"}, {From: "公寓", To: "公域"}, {From: "公域托客", To: "公域拓客"}} {
		_ = a.store.PutReplacement("", r)
	}
	a.coordinator.Active, a.coordinator.Generation, a.coordinator.Phase = true, 7, "finalizing"
	a.onSpeech(bridge.Event{Type: "final", SessionID: 7, Text: "打开石玉宝看公寓托客"})
	c := a.coordinator
	if c.Original != "打开私域宝看公域拓客" || c.Refined != c.Original || c.Phase != "inserting" {
		t.Fatalf("original %q refined %q phase %q", c.Original, c.Refined, c.Phase)
	}
}

func TestImportMessageAndReadLimited(t *testing.T) {
	if got := importMessage(vocabulary.ImportResult{TermsAdded: 2, ReplacementsAdded: 1}); got != "已导入：新增 2 个词条、1 条替换词。" {
		t.Fatalf("%q", got)
	}
	got := importMessage(vocabulary.ImportResult{TermsAdded: 1, TermsUpdated: 1, ReplacementsUpdated: 2, Skipped: 3})
	if !strings.Contains(got, "更新 1 个词条、2 条替换词") || !strings.Contains(got, "跳过 3 条") {
		t.Fatalf("%q", got)
	}
	path := filepath.Join(t.TempDir(), "v.json")
	_ = os.WriteFile(path, []byte("12345"), 0o600)
	if b, err := readLimited(path, 5); err != nil || string(b) != "12345" {
		t.Fatalf("%q %v", b, err)
	}
	if _, err := readLimited(path, 4); err == nil {
		t.Fatal("oversized file read")
	}
}
