package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/vfasky/my-ai-audio/gui/internal/asr"
	"github.com/vfasky/my-ai-audio/gui/internal/firmware"
	"github.com/vfasky/my-ai-audio/gui/internal/jev"
	"github.com/vfasky/my-ai-audio/gui/internal/permission"
	"github.com/vfasky/my-ai-audio/gui/internal/refinement"
	"github.com/vfasky/my-ai-audio/gui/internal/vocabulary"
)

type intents struct {
	scan, cancel, warmup, install, configure int
	requested, settings                      []permission.Name
	tested                                   []refinement.Config
	testKeys, configureKeys                  []string
	put                                      []vocabulary.Term
	previous                                 []string
	replaced                                 []vocabulary.Replacement
	replacedPrevious, deletedReplacements    []string
	imports, exports, quits                  int
	asrSaved, asrTested                      []asr.Config
	asrKeys                                  []string
	asrDeleted                               int
	login                                    []bool
	loginItems                               int
	jevSaved                                 []jev.Config
	jevKeys                                  []string
	jevTested                                []jev.Config
	jevDeleted                               int
}

func testView(rec *intents) *View {
	return &View{
		Snapshot: Snapshot{Install: firmware.Initial(), Config: refinement.Default()},
		Actions: Actions{
			FirmwareScan: func() {}, FirmwareInstall: func() { rec.install++ }, FirmwareSelect: func(string) {},
			Scan: func() { rec.scan++ }, Forget: func() {}, Cancel: func() { rec.cancel++ }, Warmup: func() { rec.warmup++ },
			Select: func(string) {}, Put: func(previous string, term vocabulary.Term) {
				rec.put = append(rec.put, term)
				rec.previous = append(rec.previous, previous)
			}, Delete: func(string) {},
			PutReplacement: func(previous string, r vocabulary.Replacement) {
				rec.replaced = append(rec.replaced, r)
				rec.replacedPrevious = append(rec.replacedPrevious, previous)
			},
			DeleteReplacement: func(from string) { rec.deletedReplacements = append(rec.deletedReplacements, from) },
			ImportVocabulary:  func() { rec.imports++ },
			Quit:              func() { rec.quits++ },
			ExportVocabulary:  func() { rec.exports++ },
			ConfigureASR: func(c asr.Config, key string) {
				rec.asrSaved = append(rec.asrSaved, c)
				rec.asrKeys = append(rec.asrKeys, key)
			},
			TestASR:      func(c asr.Config, _ string) { rec.asrTested = append(rec.asrTested, c) },
			DeleteASRKey: func() { rec.asrDeleted++ },
			Configure: func(_ refinement.Config, key string) {
				rec.configure++
				rec.configureKeys = append(rec.configureKeys, key)
			}, Credential: func(string, string) {}, DeleteCredential: func(string) {},
			TestRemote: func(c refinement.Config, key string) {
				rec.tested = append(rec.tested, c)
				rec.testKeys = append(rec.testKeys, key)
			},
			RequestPermission: func(n permission.Name) { rec.requested = append(rec.requested, n) },
			OpenSettings:      func(n permission.Name) { rec.settings = append(rec.settings, n) },
			SetOpenAtLogin:    func(on bool) { rec.login = append(rec.login, on) },
			OpenLoginItems:    func() { rec.loginItems++ },
			ConfigureJev: func(c jev.Config, key string) {
				rec.jevSaved = append(rec.jevSaved, c)
				rec.jevKeys = append(rec.jevKeys, key)
			},
			TestJev:      func(c jev.Config, _ string) { rec.jevTested = append(rec.jevTested, c) },
			DeleteJevKey: func() { rec.jevDeleted++ },
		},
	}
}

func newTester(v *View) *ui.Tester {
	tt := ui.NewTester(v.Draw, 1120, 840)
	tt.SetDark(true)
	tt.SetTitleBar(ui.TitleBar{Height: TitleBarHeight, Left: 78})
	return tt
}

func TestDeviceActionsFollowState(t *testing.T) {
	var rec intents
	v := testView(&rec)
	tt := newTester(v)
	// No device: the card offers a scan, and neither cancel nor the model.
	if err := tt.Click("扫描附近设备"); err != nil || rec.scan != 1 {
		t.Fatalf("scan: %v %d", err, rec.scan)
	}
	if tt.HasText("取消本次语音") || tt.HasText("重新准备语音模型") || !tt.HasText("正在准备中文语音模型") {
		t.Fatalf("idle texts %q", tt.Texts())
	}
	// A failed model offers a retry.
	v.Snapshot.Model = "zh-CN model unavailable; retry preparation"
	tt.Frame()
	if err := tt.Click("重新准备语音模型"); err != nil || rec.warmup != 1 || !tt.HasText("中文语音模型不可用") {
		t.Fatalf("retry: %v %d", err, rec.warmup)
	}
	// A session offers only cancel.
	v.Snapshot.Selected, v.Snapshot.Phase, v.Snapshot.SpeechActive = "6F1C-22A0", "recording", true
	tt.Frame()
	if tt.HasText("重新准备语音模型") || tt.HasText("扫描附近设备") || tt.HasText("重新扫描") {
		t.Fatalf("session texts %q", tt.Texts())
	}
	if err := tt.Click("取消本次语音"); err != nil || rec.cancel != 1 {
		t.Fatalf("cancel: %v %d", err, rec.cancel)
	}
	// Connected and idle: scanning moves to the device list.
	v.Snapshot.Phase, v.Snapshot.SpeechActive, v.Snapshot.Model = "ready", false, "zh-CN model warmed"
	tt.Frame()
	if tt.HasText("取消本次语音") || tt.HasText("重新扫描") || !tt.HasText("中文语音模型已就绪") {
		t.Fatalf("connected texts %q", tt.Texts())
	}
	if err := tt.Click("扫描"); err != nil || rec.scan != 2 {
		t.Fatalf("list scan: %v %d", err, rec.scan)
	}
	// Disconnected with a remembered device: the card rescans.
	v.Snapshot.Phase = "disconnected"
	tt.Frame()
	if err := tt.Click("重新扫描"); err != nil || rec.scan != 3 {
		t.Fatalf("rescan: %v %d", err, rec.scan)
	}
}

func TestModelStatus(t *testing.T) {
	for message, want := range map[string]modelState{
		"": modelPreparing, "SpeechBridge ready": modelPreparing, "Checking zh-CN assets": modelPreparing,
		"Downloading zh-CN model": modelPreparing, "zh-CN model warmed": modelReady,
		"zh-CN model unavailable; retry preparation": modelFailed,
	} {
		if _, got := modelStatus(message); got != want {
			t.Errorf("%q: %v, want %v", message, got, want)
		}
	}
}

func TestSidebarNavigatesAndInstallIsGated(t *testing.T) {
	var rec intents
	v := testView(&rec)
	tt := newTester(v)
	if err := tt.Click("固件安装"); err != nil || v.Page != 4 {
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
	v.Page = 3
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
	if !tt.HasText(modeHelp("light")) {
		t.Fatalf("light help missing: %q", tt.Texts())
	}
	if err := tt.Click("结构化"); err != nil || v.Config.Mode != "structured" || !tt.HasText(modeHelp("structured")) {
		t.Fatalf("mode: %v %q", err, v.Config.Mode)
	}
	if !tt.HasText("显式远端模式：最终转写和词库将发送到所填 HTTPS 端点；音频不发送。") {
		t.Fatal("remote disclosure missing")
	}
	if err := tt.Click("保存设置"); err != nil || rec.configure != 1 {
		t.Fatalf("save: %v", err)
	}
}

func TestRemoteConnectionTest(t *testing.T) {
	var rec intents
	v := testView(&rec)
	v.Page = 3
	tt := newTester(v)
	// Typed on the page, not saved yet.
	v.Config.Endpoint, v.Config.Model, v.secret = "https://provider.example/v1", "m", "typed"
	tt.Frame()
	// The draft on the page is tested, with the typed key, before saving.
	if err := tt.Click("测试连接"); err != nil || len(rec.tested) != 1 || rec.tested[0].Model != "m" || rec.testKeys[0] != "typed" || rec.configure != 0 {
		t.Fatalf("test: %v %+v", err, rec)
	}
	v.Snapshot.Testing = true
	tt.Frame()
	_ = tt.Click("正在测试…")
	if len(rec.tested) != 1 || tt.HasText("测试连接") {
		t.Fatal("a running test started again")
	}
	v.Snapshot.Testing, v.Snapshot.Feedback, v.Snapshot.FeedbackOK, v.Snapshot.FeedbackFor = false, "API key 无效或没有权限（HTTP 401）。", false, v.Config
	tt.Frame()
	if !tt.HasText("API key 无效或没有权限（HTTP 401）。") {
		t.Fatalf("result missing: %q", tt.Texts())
	}
	// Editing the draft hides a result that no longer describes it.
	v.Config.Model = "other"
	tt.Frame()
	if tt.HasText("API key 无效或没有权限（HTTP 401）。") {
		t.Fatal("stale result shown")
	}
}

func TestSaveSettingsHandsOverTypedKey(t *testing.T) {
	var rec intents
	v := testView(&rec)
	v.Page = 3
	tt := newTester(v)
	v.Config.Endpoint, v.secret = "https://provider.example/v1", "typed"
	tt.Frame()
	if err := tt.Click("保存设置"); err != nil || rec.configure != 1 || rec.configureKeys[0] != "typed" || v.secret != "" {
		t.Fatalf("save: %v %+v %q", err, rec, v.secret)
	}
	v.Snapshot.Feedback, v.Snapshot.FeedbackOK, v.Snapshot.FeedbackFor = "设置已保存，API key 已存入钥匙串。", true, v.Config
	tt.Frame()
	if !tt.HasText("设置已保存，API key 已存入钥匙串。") {
		t.Fatalf("feedback missing: %q", tt.Texts())
	}
	// Without an endpoint the typed key stays in the field.
	v.Config.Endpoint, v.secret = "", "typed"
	tt.Frame()
	if err := tt.Click("保存到钥匙串"); err != nil || v.secret != "typed" {
		t.Fatalf("key without endpoint: %v %q", err, v.secret)
	}
}

func TestVocabularyTabsAndReplacements(t *testing.T) {
	var rec intents
	v := testView(&rec)
	v.Page = 1
	v.Snapshot.Terms = []vocabulary.Term{{Term: "私域宝", NormalizedTerm: "私域宝", Source: "manual", Weight: 50}}
	tt := newTester(v)
	if !tt.HasText("词条 · 1") || !tt.HasText("替换词 · 0") || !tt.HasText("私域宝") {
		t.Fatalf("terms tab %q", tt.Texts())
	}
	if err := tt.Click("替换词 · 0"); err != nil || v.vocabTab != 1 {
		t.Fatalf("tab: %v %d", err, v.vocabTab)
	}
	// Both fields are needed.
	v.from = "施玉宝"
	tt.Frame()
	_ = tt.Click("保存")
	if len(rec.replaced) != 0 {
		t.Fatal("saved without a target")
	}
	v.to = "私域宝"
	tt.Frame()
	if err := tt.Click("保存"); err != nil || len(rec.replaced) != 1 || rec.replaced[0] != (vocabulary.Replacement{From: "施玉宝", To: "私域宝"}) || v.from != "" || v.to != "" {
		t.Fatalf("save: %v %+v", err, rec.replaced)
	}
	v.Snapshot.Replacements = []vocabulary.Replacement{{From: "施玉宝", To: "私域宝"}}
	tt.Frame()
	if !tt.HasText("施玉宝") || !tt.HasText("替换词 · 1") {
		t.Fatalf("list %q", tt.Texts())
	}
	// Editing loads both fields and replaces the one it edits.
	if err := tt.Click("编辑"); err != nil || v.from != "施玉宝" || v.to != "私域宝" {
		t.Fatalf("edit: %v %q %q", err, v.from, v.to)
	}
	v.from = "施域宝"
	tt.Frame()
	if err := tt.Click("更新"); err != nil || len(rec.replaced) != 2 || rec.replacedPrevious[1] != "施玉宝" || rec.replaced[1].From != "施域宝" {
		t.Fatalf("update: %v %+v", err, rec)
	}
	if err := tt.Click("删除"); err != nil || len(rec.deletedReplacements) != 1 || rec.deletedReplacements[0] != "施玉宝" {
		t.Fatalf("delete: %v %+v", err, rec.deletedReplacements)
	}
	// Back to the terms.
	if err := tt.Click("词条 · 1"); err != nil || v.vocabTab != 0 || !tt.HasText("添加词条") {
		t.Fatalf("terms: %v", err)
	}
}

func TestVocabularyImportExport(t *testing.T) {
	var rec intents
	v := testView(&rec)
	v.Page = 1
	tt := newTester(v)
	if err := tt.Click("导入"); err != nil || rec.imports != 1 {
		t.Fatalf("import: %v", err)
	}
	if err := tt.Click("导出"); err != nil || rec.exports != 1 {
		t.Fatalf("export: %v", err)
	}
	v.Snapshot.VocabFeedback, v.Snapshot.VocabFeedbackOK = "已导入：新增 2 个词条、1 条替换词。", true
	v.vocabTab = 1
	tt.Frame()
	if !tt.HasText("已导入：新增 2 个词条、1 条替换词。") {
		t.Fatalf("feedback %q", tt.Texts())
	}
}

func TestRecognitionDropdownChoosesOnlineModel(t *testing.T) {
	var rec intents
	v := testView(&rec)
	v.Page = 2
	tt := newTester(v)
	// Local by default: no key, no test, no disclosure.
	if !tt.HasText("Apple 本地（SpeechAnalyzer）") || tt.HasText("测试连接") || tt.HasText("删除凭据") {
		t.Fatalf("local texts %q", tt.Texts())
	}
	if err := tt.Click("选择识别引擎"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("支付宝 MaaS · Fun-ASR 实时"); err != nil || v.ASR != asr.Choices[1].Config {
		t.Fatalf("choose: %v %+v", err, v.ASR)
	}
	if !tt.HasText("在线识别：每次语音的音频会实时发送到支付宝 MaaS（maas-api.antdigital.com）转写，不发送词库，也不在本机保存音频。连接失败时本次语音结束，不会自动改用本地识别。") {
		t.Fatalf("online disclosure missing: %q", tt.Texts())
	}
	v.asrSecret = "typed"
	tt.Frame()
	if err := tt.Click("测试连接"); err != nil || len(rec.asrTested) != 1 || rec.asrTested[0].Model != "fun-asr-realtime" {
		t.Fatalf("test: %v %+v", err, rec.asrTested)
	}
	if err := tt.Click("保存设置"); err != nil || len(rec.asrSaved) != 1 || rec.asrKeys[0] != "typed" || v.asrSecret != "" {
		t.Fatalf("save: %v %+v %q", err, rec, v.asrSecret)
	}
	if err := tt.Click("删除凭据"); err != nil || rec.asrDeleted != 1 {
		t.Fatalf("delete: %v", err)
	}
	// Feedback shows for the draft it describes only.
	v.Snapshot.ASRFeedback, v.Snapshot.ASRFeedbackOK, v.Snapshot.ASRFeedbackFor = "连接成功", true, v.ASR
	tt.Frame()
	if !tt.HasText("连接成功") {
		t.Fatal("feedback missing")
	}
	v.ASR = asr.Choices[2].Config
	tt.Frame()
	if tt.HasText("连接成功") || !tt.HasText("支付宝 MaaS · Paraformer 实时 v2") {
		t.Fatalf("stale feedback or label: %q", tt.Texts())
	}
}

func TestRecognitionDropdownChoosesWholeRecording(t *testing.T) {
	var rec intents
	v := testView(&rec)
	v.Page = 2
	tt := newTester(v)
	_ = tt.Click("选择识别引擎")
	if err := tt.Click("支付宝 MaaS · Qwen3-ASR-Flash（非流式）"); err != nil || v.ASR != asr.Choices[3].Config {
		t.Fatalf("choose: %v %+v", err, v.ASR)
	}
	if !tt.HasText("在线识别：每次语音的音频会在松开按键后整段发送到支付宝 MaaS") || !tt.HasText(asrHelp(v.ASR)) || !tt.HasText("测试连接") {
		t.Fatalf("texts %q", tt.Texts())
	}
}

func TestRecognitionOmniKeepsProviderCredentialsSeparate(t *testing.T) {
	var rec intents
	v := testView(&rec)
	v.Page = 2
	v.ASR = asr.Choices[1].Config
	v.asrSecret = "alipay-key"
	tt := newTester(v)
	_ = tt.Click("选择识别引擎")
	if err := tt.Click("阿里云百炼 · Omni Realtime（WebSocket）"); err != nil || v.ASR.Engine != asr.EngineOmni {
		t.Fatalf("choose Omni: %v %+v", err, v.ASR)
	}
	if v.asrSecret != "" || !tt.HasText("WebSocket 端点") || !tt.HasText("阿里云百炼") || !tt.HasText(asrHelp(v.ASR)) {
		t.Fatalf("Omni form or previous provider key: %q", tt.Texts())
	}
	v.ASR.Endpoint = "wss://workspace.cn-beijing.maas.aliyuncs.com/api-ws/v1/realtime"
	v.asrSecret = "omni-key"
	tt.Frame()
	if err := tt.Click("测试连接"); err != nil || len(rec.asrTested) != 1 || rec.asrTested[0] != v.ASR {
		t.Fatalf("test custom endpoint: %v %+v", err, rec.asrTested)
	}
	if err := tt.Click("保存设置"); err != nil || len(rec.asrSaved) != 1 || rec.asrSaved[0] != v.ASR || rec.asrKeys[0] != "omni-key" {
		t.Fatalf("save Omni: %v %+v", err, rec)
	}
}

func TestSidebarQuitsUnlessInstalling(t *testing.T) {
	var rec intents
	v := testView(&rec)
	tt := newTester(v)
	if err := tt.Click("退出 go-voice"); err != nil || rec.quits != 1 {
		t.Fatalf("quit: %v %d", err, rec.quits)
	}
	v.Snapshot.Install.Phase = "writing"
	if !v.Snapshot.Install.Busy() {
		t.Skip("writing is not a busy install phase")
	}
	tt.Frame()
	_ = tt.Click("退出 go-voice")
	if rec.quits != 1 {
		t.Fatal("quit during a firmware install")
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
	s.Connection, s.Model, s.Status = "Connected; ATT payload ≥182 B", "zh-CN model warmed", "Credential saved in Keychain"
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

func TestFirmwarePageShowsVersionsAndUpdate(t *testing.T) {
	var rec intents
	v := testView(&rec)
	v.Page = 4
	v.Snapshot.Firmware = "0.1.0"
	v.Snapshot.BundledFirmware = "0.2.0"
	tt := newTester(v)
	if !tt.HasText("0.1.0") || !tt.HasText("0.2.0") {
		t.Fatalf("versions missing: %q", tt.Texts())
	}
	if !tt.HasText("设备固件 0.1.0 低于随附 0.2.0，建议安装。") {
		t.Fatalf("update notice missing: %q", tt.Texts())
	}
	if !tt.HasText("更新") {
		t.Fatalf("sidebar badge missing: %q", tt.Texts())
	}
	v.Snapshot.Firmware = "0.2.0"
	tt = newTester(v)
	if !tt.HasText("设备已是随附固件 0.2.0。") || tt.HasText("建议安装") {
		t.Fatalf("current notice: %q", tt.Texts())
	}
}

func TestFirmwareErrorShowsCodeAndStage(t *testing.T) {
	var rec intents
	v := testView(&rec)
	v.Page = 4
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

func TestGeneralPageTogglesOpenAtLogin(t *testing.T) {
	var rec intents
	v := testView(&rec)
	tt := newTester(v)
	if err := tt.Click("通用"); err != nil || v.Page != GeneralPage {
		t.Fatalf("navigate: %v page %d", err, v.Page)
	}
	if err := tt.Click("登录时打开 go-voice"); err != nil || len(rec.login) != 1 || !rec.login[0] {
		t.Fatalf("enable: %v %v", err, rec.login)
	}
	v.Snapshot.OpenAtLogin = true
	tt.Frame()
	if err := tt.Click("登录时打开 go-voice"); err != nil || len(rec.login) != 2 || rec.login[1] {
		t.Fatalf("disable: %v %v", err, rec.login)
	}
	v.Snapshot.OpenAtLogin, v.Snapshot.LoginApproval = false, true
	tt.Frame()
	if err := tt.Click("打开登录项"); err != nil || rec.loginItems != 1 {
		t.Fatalf("login items: %v %d", err, rec.loginItems)
	}
}

func TestVocabularyJevTab(t *testing.T) {
	var rec intents
	v := testView(&rec)
	v.Page = 1
	tt := newTester(v)
	if err := tt.Click("Jev 判断"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("api.typesafe.ai") {
		t.Fatal("privacy notice while off")
	}
	if err := tt.Click("启用 Jev 词库判断"); err != nil || !v.Jev.Enabled {
		t.Fatalf("switch: %v %+v", err, v.Jev)
	}
	if !tt.HasText("api.typesafe.ai") {
		t.Fatalf("privacy notice missing: %q", tt.Texts())
	}
	if err := tt.Click("支付宝 MaaS"); err != nil || v.Jev.Endpoint != "https://maas-api.antdigital.com/v1/systemone" || v.Jev.Model != "jev-1.13" {
		t.Fatalf("preset: %v %+v", err, v.Jev)
	}
	tt.Frame()
	if !tt.HasText("发送到 maas-api.antdigital.com") {
		t.Fatalf("privacy notice names the provider: %q", tt.Texts())
	}
	if err := tt.Click("测试连接"); err != nil || len(rec.jevTested) != 1 || rec.jevTested[0].Model != "jev-1.13" {
		t.Fatalf("test: %v %v", err, rec.jevTested)
	}
	if err := tt.Click("保存设置"); err != nil || len(rec.jevSaved) != 1 || !rec.jevSaved[0].Enabled || rec.jevSaved[0].Endpoint != "https://maas-api.antdigital.com/v1/systemone" {
		t.Fatalf("save: %v %+v", err, rec.jevSaved)
	}
	v.Snapshot.JevFeedback, v.Snapshot.JevFeedbackOK, v.Snapshot.JevFeedbackFor = "设置已保存", true, v.Jev
	tt.Frame()
	if !tt.HasText("设置已保存") {
		t.Fatalf("feedback: %q", tt.Texts())
	}
	if err := tt.Click("删除凭据"); err != nil || rec.jevDeleted != 1 {
		t.Fatalf("delete: %v %d", err, rec.jevDeleted)
	}
}
