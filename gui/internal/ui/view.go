// Package ui renders snapshots and emits user intentions; it owns no native sessions.
package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
	"github.com/vfasky/my-ai-audio/gui/internal/firmware"
	"github.com/vfasky/my-ai-audio/gui/internal/permission"
	"github.com/vfasky/my-ai-audio/gui/internal/protocol"
	"github.com/vfasky/my-ai-audio/gui/internal/refinement"
	"github.com/vfasky/my-ai-audio/gui/internal/vocabulary"
)

type Device struct{ ID, Name string }
type Snapshot struct {
	Install                                                                          firmware.State
	SpeechActive                                                                     bool
	Devices                                                                          []Device
	Selected, Connection, Firmware, Phase, Partial, Original, Refined, Status, Model string
	Ready                                                                            bool
	Meter                                                                            protocol.Meter
	Terms                                                                            []vocabulary.Term
	Config                                                                           refinement.Config
	Permissions                                                                      permission.Set
	// CanCommit: the dictation capsule may insert the ASR original now.
	CanCommit bool
}
type Actions struct {
	FirmwareScan, FirmwareInstall func()
	FirmwareSelect                func(string)
	Scan, Forget, Cancel, Warmup  func()
	Select                        func(string)
	Put                           func(string, vocabulary.Term)
	Delete                        func(string)
	Configure                     func(refinement.Config)
	Credential                    func(string, string)
	DeleteCredential              func(string)
	RequestPermission             func(permission.Name)
	OpenSettings                  func(permission.Name)
	// Commit skips refinement and inserts the ASR original.
	Commit func()
}
type View struct {
	Page                  int
	Snapshot              Snapshot
	Actions               Actions
	term, editing, secret string
	locked                bool
	Config                refinement.Config
	Loaded                bool
	// AppIcon is the packaged app icon shown at the top of the sidebar.
	AppIcon *ui.Bitmap
}

// TitleBarHeight is the inset title bar the window reserves for the traffic
// lights; the toolbar and the sidebar header align to it.
const TitleBarHeight = 52

const sidebarWidth = 236

var pages = []struct {
	id, title, subtitle string
	icon                *ui.SVG
}{
	{"voice", "设备与语音", "从 FoloToy AI Passport 接收音频并在本机转写", iconWave},
	{"vocabulary", "词库", "下一次语音开始时注入 Top 100", iconBook},
	{"refinement", "AI 润色", "本地优先，失败时保留 ASR 原文", iconSparkle},
	{"firmware", "固件安装", "通过 USB 安装随应用提供的固件", iconChip},
	{"permissions", "权限", "开始使用前授予系统权限，避免使用中途失败", iconShield},
}

var (
	engines      = []string{"local", "remote"}
	engineLabels = []string{"Apple 本地", "OpenAI 兼容接口"}
	apis         = []string{refinement.APIChat, refinement.APIResponses}
	apiLabels    = []string{"Chat Completions", "Responses"}
	apiPaths     = []string{"/chat/completions", "/responses"}
	modes        = []string{"off", "names", "light", "structured"}
	modeLabels   = []string{"关闭", "专名", "轻度", "结构化"}
)

func (v *View) Draw(c *ui.Context) {
	if !v.Loaded {
		v.Config = v.Snapshot.Config
		v.Loaded = true
	}
	c.SetTheme(theme)
	c.Root().Background(colCanvas)
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		v.sidebar(c)
		v.pane(c)
	})
}

// sidebar is the floating Liquid Glass source list of macOS 26, inset from
// the window edges with the traffic lights inside it.
func (v *View) sidebar(c *ui.Context) {
	ui.Column(c).Width(sidebarWidth).Shrink(0).Padding(8, 0, 8, 8).Children(func() {
		ui.Column(c).Grow(1).Radius(20).Material(glass.Glass{Tint: ui.RGBA(0, 0, 0, 0.45)}).Border(1, colHairline).Clip().Children(func() {
			ui.Box(c).Height(TitleBarHeight - 8).Shrink(0).DragWindow()
			ui.Row(c).Padding(2, 16, 14).Gap(10).Children(func() {
				if v.AppIcon != nil {
					ui.Image(c, v.AppIcon).Size(36, 36).Fit(ui.Contain).Label("go-voice")
				} else {
					ui.Box(c).Size(30, 30).Radius(9).Background(colWell).Border(1, colHairline).Center().Children(func() {
						ui.Icon(c, iconWave).Size(16, 16).TextColor(colOrange)
					})
				}
				ui.Column(c).Gap(1).Children(func() {
					ui.Text(c, "go-voice").FontSize(14).FontWeight(600).SingleLine()
					ui.Text(c, "FoloToy AI Passport").FontSize(11).TextColor(colMuted).SingleLine()
				})
			})
			nav := pages[min(max(v.Page, 0), len(pages)-1)].id
			list := ui.Sidebar(c, &nav, func() {
				ui.SidebarSection(c, "工作区", nil, func() {
					for _, p := range pages {
						item := ui.SidebarItem(c, p.id, p.icon, p.title)
						if p.id == "vocabulary" && len(v.Snapshot.Terms) > 0 {
							item.Children(func() {
								ui.Badge(c, fmt.Sprint(len(v.Snapshot.Terms))).Background(colPill).TextColor(colMuted)
							})
						}
						if missing := len(v.Snapshot.Permissions.Missing()); p.id == "permissions" && missing > 0 {
							item.Children(func() {
								ui.Badge(c, fmt.Sprint(missing)).Background(colOrange).TextColor(ui.Hex("#FFFFFF"))
							})
						}
					}
				})
			}).Grow(1).Background(ui.Transparent).Label("导航")
			if list.Changed() {
				for i, p := range pages {
					if p.id == nav {
						v.Page = i
					}
				}
			}
			v.connectionCard(c)
		})
	})
}

func (v *View) connectionCard(c *ui.Context) {
	s := v.Snapshot
	dot, title := colFaint, "未连接"
	switch {
	case s.Install.Busy():
		dot, title = colOrange, "固件安装中"
	case s.Ready:
		dot, title = colGreen, "已连接"
	case s.Selected != "":
		dot, title = colOrange, "等待设备"
	}
	ui.Column(c).Margin(0, 10, 10).Padding(10, 12).Gap(4).Radius(14).Background(ui.RGBA(0, 0, 0, 0.28)).Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			statusDot(c, dot)
			ui.Text(c, title).FontSize(12).FontWeight(600).SingleLine()
		})
		detail := "固件 " + fallback(s.Firmware, "未知")
		if s.Selected == "" {
			detail = "尚未选择设备"
		}
		ui.Text(c, detail).FontSize(11).TextColor(colMuted).SingleLine()
	})
}

// pane is the content column: a scroll view under a toolbar that floats over
// the macOS 26 soft scroll edge.
func (v *View) pane(c *ui.Context) {
	bar := c.TitleBar()
	page := pages[min(max(v.Page, 0), len(pages)-1)]
	ui.Box(c).Grow(1).MinWidth(0).Children(func() {
		ui.Scroll(c).Fill().ScrollbarInsets(TitleBarHeight, 0, 0).Children(func() {
			ui.Column(c).Padding(TitleBarHeight+18, 36, 72).AlignItems(ui.Center).Children(func() {
				ui.Column(c).FillWidth().MaxWidth(820).Gap(28).Children(func() {
					switch v.Page {
					case 0:
						v.device(c)
					case 1:
						v.vocabulary(c)
					case 2:
						v.refinement(c)
					case 3:
						v.firmware(c)
					case PermissionsPage:
						v.permissions(c)
					}
				})
			})
		})
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(TitleBarHeight + 10).PassThrough().Material(glass.ScrollEdge{})
		ui.Row(c).Absolute().Top(0).Left(0).Right(0).Height(TitleBarHeight).Padding(0, bar.Right+16, 0, 28).Gap(12).DragWindow().Children(func() {
			ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, page.title).FontSize(15).FontWeight(650).SingleLine()
				ui.Text(c, page.subtitle).FontSize(11).TextColor(colMuted).SingleLine()
			})
			ui.Row(c).Height(30).PaddingX(12).Gap(8).Radius(15).Material(glass.Glass{}).Children(func() {
				ready := strings.Contains(strings.ToLower(v.Snapshot.Model), "ready")
				statusDot(c, pick(ready, colGreen, colFaint))
				ui.Text(c, "zh-CN · "+fallback(v.Snapshot.Model, "准备中")).FontSize(11.5).TextColor(colSubtle).SingleLine().MaxWidth(220)
			})
		})
		if v.Snapshot.Status != "" {
			ui.Box(c).Absolute().Bottom(0).Left(0).Right(0).Height(64).PassThrough().Material(glass.ScrollEdge{Bottom: true})
			ui.Row(c).Attach(ui.AnchorBottom, ui.AnchorBottom).Bottom(16).Height(32).PaddingX(14).Gap(8).Radius(16).
				Material(glass.Glass{}).PassThrough().Children(func() {
				ui.Text(c, v.Snapshot.Status).FontSize(12).TextColor(colSubtle).SingleLine().MaxWidth(520)
			})
		}
	})
}

func phaseLabel(p string) string {
	switch p {
	case "ready":
		return "就绪 · 按下 FoloToy AI Passport 按键开始说话"
	case "starting":
		return "正在开始"
	case "recording":
		return "正在聆听"
	case "tail":
		return "正在收尾"
	case "finalizing":
		return "正在识别"
	case "refining":
		return "AI 润色中"
	case "inserting":
		return "正在插入文字"
	case "disconnected":
		return "已断开"
	}
	return "等待连接"
}

func isLive(p string) bool { return p == "starting" || p == "recording" || p == "tail" }
func isWorking(p string) bool {
	return p == "finalizing" || p == "refining" || p == "inserting"
}

func (v *View) deviceName() string {
	s := v.Snapshot
	if s.Selected == "" {
		return "未选择 FoloToy AI Passport"
	}
	for _, d := range s.Devices {
		if d.ID == s.Selected && d.Name != "" {
			return d.Name
		}
	}
	return "FoloToy AI Passport"
}

func (v *View) device(c *ui.Context) {
	s := v.Snapshot
	if missing := s.Permissions.Missing(); len(missing) > 0 {
		v.permissionBanner(c, missing)
	}
	ui.Row(c).Gap(16).Wrap().GapY(12).Children(func() {
		if pill(c, "warmup", "准备 zh-CN 模型", pillSecondary, iconPlay, nil, s.SpeechActive) {
			v.Actions.Warmup()
		}
		ui.Box(c).Grow(1).MinWidth(40).Height(1).Background(colRule)
		if pill(c, "scan", "扫描 FoloToy AI Passport", pillPrimary, nil, iconArrow, s.Install.Busy()) {
			v.Actions.Scan()
		}
		if pill(c, "cancel", "取消本次语音", pillSecondary, nil, iconClose, !s.SpeechActive) {
			v.Actions.Cancel()
		}
	})
	ui.RichText(c,
		ui.Span{Text: "仅使用 "},
		ui.Span{Text: "FoloToy AI Passport 音频", Weight: 650},
		ui.Span{Text: "在本机实时转写。不保存、不回放音频，也不使用 Mac 麦克风；最终文本插入到你选择的应用。"},
	).FontSize(16).LineHeight(1.65).TextColor(colSubtle)

	live, working := isLive(s.Phase), isWorking(s.Phase)
	ui.Column(c).Background(colCard).Radius(26).Padding(28, 28, 26).Gap(24).Children(func() {
		ui.Column(c).AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Text(c, v.deviceName()).FontSize(19).FontWeight(500).SingleLine()
			sub := phaseLabel(s.Phase)
			if s.Firmware != "" {
				sub += " · 固件 " + s.Firmware
			}
			ui.Text(c, sub).FontSize(14).TextColor(colSubtle).SingleLine()
		})
		// One well follows the session: idle dots, live bars, then a pulse while
		// recognition, refinement and insertion run.
		meter := s.Meter
		switch {
		case working:
			well(c, phaseLabel(s.Phase), true, func(p *ui.Painter, r ui.Rect) { drawPulse(p, r, true) })
		case live:
			well(c, "实时音频", true, func(p *ui.Painter, r ui.Rect) { drawWave(p, r, meter.Bins, true) })
		default:
			well(c, "等待语音", false, func(p *ui.Painter, r ui.Rect) { drawWave(p, r, meter.Bins, false) })
		}
		ui.Row(c).Justify(ui.Center).Gap(36).Children(func() {
			stat(c, "时长", fmt.Sprintf("%.1f s", s.Meter.Duration().Seconds()))
			stat(c, "RMS", fmt.Sprintf("%.3f", s.Meter.RMS))
			stat(c, "PEAK", fmt.Sprintf("%.3f", s.Meter.Peak))
		})
	})

	notice(c, "v1", "V1 未认证连接：设备 UUID 仅用于选择，不证明设备身份。", true)

	group(c, "transcript", "本次语音", nil, func() {
		for _, pair := range [][2]string{{"实时转写", s.Partial}, {"最终 ASR 原文", s.Original}, {"润色结果", s.Refined}} {
			row(c, pair[0], func() {
				ui.Text(c, pair[0]).FontSize(13).TextColor(colMuted).Width(96).Shrink(0).AlignSelf(ui.Start)
				t := ui.Text(c, fallback(pair[1], "—")).FontSize(14).LineHeight(1.5).MaxLines(10).Grow(1).Selectable()
				if pair[1] == "" {
					t.TextColor(colFaint)
				}
			})
		}
	})

	group(c, "devices", "附近的 FoloToy AI Passport", func() {
		if s.Selected != "" {
			if smallPill(c, "forget", "忘记设备", pillGhost, nil, false) {
				v.Actions.Forget()
			}
		}
	}, func() {
		if len(s.Devices) == 0 {
			row(c, "empty", func() {
				ui.Text(c, "尚未发现设备。点击“扫描 FoloToy AI Passport”开始搜索附近的设备。").FontSize(13).TextColor(colMuted)
			})
		}
		for _, d := range s.Devices {
			row(c, "ble-"+d.ID, func() {
				ui.Box(c).Size(30, 30).Radius(15).Background(colPill).Center().Children(func() {
					ui.Icon(c, iconBolt).Size(14, 14).TextColor(pick(d.ID == s.Selected, colOrange, colMuted))
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, fallback(d.Name, "FoloToy AI Passport")).FontSize(14).FontWeight(500).SingleLine()
					ui.Text(c, d.ID).FontSize(11).TextColor(colFaint).SingleLine().FontFeatures("tnum")
				})
				if d.ID == s.Selected {
					ui.Badge(c, "已选择").Background(colOrangeSoft).TextColor(colOrange)
				} else if smallPill(c, "select", "选择并连接", pillSecondary, nil, s.Install.Busy()) {
					v.Actions.Select(d.ID)
				}
			})
		}
	})

	group(c, "diagnostics", "诊断", nil, func() {
		ui.Row(c).Padding(14, 16).Gap(10).Children(func() {
			for _, m := range []struct {
				label string
				value uint32
			}{{"收帧", s.Meter.Received}, {"缺帧", s.Meter.Missing}, {"补帧", s.Meter.Recovered}, {"重复", s.Meter.Duplicates}} {
				ui.Column(c).Grow(1).Basis(0).Padding(10, 12).Gap(4).Radius(12).Background(colWell).Children(func() {
					ui.Text(c, m.label).FontSize(11).TextColor(colMuted)
					ui.Text(c, fmt.Sprint(m.value)).FontSize(18).FontWeight(500).FontFeatures("tnum")
				})
			}
		})
		kv(c, "连接", fallback(s.Connection, "—"))
		kv(c, "握手", pick2(s.Ready, "已完成", "未完成"))
		kv(c, "语音模型", fallback(s.Model, "—"))
	})

	ui.Text(c, "插字需要辅助功能与自动化授权。Secure Input 或未授权时，最终文本仅保留在剪贴板。").FontSize(12).TextColor(colMuted).LineHeight(1.5)
}

func stat(c *ui.Context, label, value string) {
	ui.Column(c).AlignItems(ui.Center).Gap(4).Children(func() {
		ui.Text(c, value).FontSize(15).FontWeight(500).FontFeatures("tnum").SingleLine()
		spaced(c, label, colMuted).FontSize(10.5).LetterSpacing(1.6)
	})
}

func kv(c *ui.Context, key, value string) {
	row(c, key, func() {
		ui.Text(c, key).FontSize(13).TextColor(colMuted).Grow(1)
		ui.Text(c, value).FontSize(13).TextColor(colSubtle).SingleLine().MaxWidth(420)
	})
}

func (v *View) firmware(c *ui.Context) {
	s := v.Snapshot.Install
	tone := colMuted
	switch {
	case s.Phase == "error":
		tone = colRed
	case s.Phase == "complete":
		tone = colGreen
	case s.Busy():
		tone = colOrange
	}
	ui.Column(c).Background(colCard).Radius(26).Padding(26, 28).Gap(18).Children(func() {
		ui.Row(c).Gap(16).Children(func() {
			ui.Box(c).Size(52, 52).Radius(16).Background(colWell).Border(1, colHairline).Center().Children(func() {
				ui.Icon(c, iconChip).Size(24, 24).TextColor(tone)
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				ui.Text(c, firmware.StageLabel(s.Phase)).FontSize(19).FontWeight(500)
				ui.Text(c, fallback(s.Message, "仅安装随应用提供的 Go Voice 固件。")).FontSize(13).LineHeight(1.5).TextColor(colSubtle).MaxLines(6)
				if s.Phase == "error" && s.ErrorCode != "" {
					// Helper codes are fixed identifiers, never device or path data.
					ui.Text(c, fmt.Sprintf("错误码 %s · 失败阶段：%s", s.ErrorCode, firmware.StageLabel(s.FailedStage))).
						FontSize(12).TextColor(colRed).FontFeatures("tnum").Selectable()
				}
			})
		})
		if s.Busy() || s.Phase == "complete" {
			ui.Column(c).Gap(8).Children(func() {
				ui.Progress(c, float64(s.Percent)/100).Label("当前阶段进度")
				ui.Text(c, fmt.Sprintf("当前阶段进度 %d%%", s.Percent)).FontSize(12).TextColor(colMuted).FontFeatures("tnum")
			})
		}
	})

	notice(c, "lock", "安装前会断开当前蓝牙设备；写入开始后无法取消。写入、验证和重启期间请勿拔线或退出应用。", true)
	notice(c, "cable", "请使用可靠的 USB 数据线，并确认选中的设备。不显示或保存 USB 序列号；USB 选择不证明设备身份，也不会自动绑定蓝牙设备。", false)

	group(c, "usb", "USB 设备", func() {
		if smallPill(c, "usb-scan", "扫描 USB 设备", pillSecondary, nil, !s.CanScan()) {
			v.Actions.FirmwareScan()
		}
	}, func() {
		if len(s.Devices) == 0 {
			row(c, "usb-empty", func() {
				ui.Text(c, "未发现兼容设备。连接 FoloToy AI Passport 后点击“扫描 USB 设备”。").FontSize(13).TextColor(colMuted)
			})
		}
		for i, d := range s.Devices {
			row(c, fmt.Sprintf("usb-%d", i), func() {
				// Helper names and port paths can contain serial numbers: never render them.
				ui.Icon(c, iconChip).Size(16, 16).TextColor(pick(d.ID == s.Selected, colOrange, colMuted))
				ui.Text(c, fmt.Sprintf("兼容 USB 设备 %d", i+1)).FontSize(14).Grow(1)
				if d.ID == s.Selected {
					ui.Badge(c, "已选择").Background(colOrangeSoft).TextColor(colOrange)
				} else if s.Phase == "ready" {
					if smallPill(c, "usb-select", "选择此设备", pillSecondary, nil, false) {
						v.Actions.FirmwareSelect(d.ID)
					}
				}
			})
		}
	})
	if len(s.Devices) > 1 {
		notice(c, "many", "存在多个兼容设备。为避免选错，请仅连接要安装固件的设备后重新扫描。", true)
	}
	if v.Snapshot.SpeechActive {
		notice(c, "speech", "语音会话进行中，请先完成或取消本次语音。", true)
	}
	ui.Row(c).Gap(16).Children(func() {
		ui.Box(c).Grow(1).Height(1).Background(colRule)
		if s.Phase == "complete" {
			if pill(c, "goto-device", "前往设备页选择刚重启的 FoloToy AI Passport", pillSecondary, nil, iconArrow, false) {
				v.Page = 0
			}
		}
		if pill(c, "install", "确认安装随附固件", pillPrimary, iconDownload, nil, !s.CanInstall(v.Snapshot.SpeechActive)) {
			v.Actions.FirmwareInstall()
		}
	})
}

func (v *View) vocabulary(c *ui.Context) {
	notice(c, "vocab", "动态词库在下一次语音开始时应用 Top 100；锁定词不能被 AI 自动删除。", false)
	title := "添加词条"
	if v.editing != "" {
		title = "编辑词条"
	}
	group(c, "editor", title, nil, func() {
		row(c, "input", func() {
			ui.TextInput(c, &v.term).Placeholder("术语 / 标准写法").Grow(1).Label("术语")
			ui.Row(c).Gap(8).Children(func() {
				ui.Icon(c, iconLock).Size(14, 14).TextColor(colMuted)
				ui.Text(c, "锁定").FontSize(13).TextColor(colSubtle)
				ui.Switch(c, &v.locked).Label("锁定")
			})
			if v.editing != "" {
				if smallPill(c, "cancel-edit", "取消", pillGhost, nil, false) {
					v.term, v.editing, v.locked = "", "", false
				}
			}
			if smallPill(c, "save-term", pick2(v.editing != "", "更新", "保存"), pillPrimary, iconPlus, strings.TrimSpace(v.term) == "") {
				v.Actions.Put(v.editing, vocabulary.Term{Term: v.term, Source: "manual", Weight: 50, Locked: v.locked})
				v.term = ""
				v.editing = ""
			}
		})
	})
	group(c, "terms", fmt.Sprintf("词条 · %d", len(v.Snapshot.Terms)), nil, func() {
		if len(v.Snapshot.Terms) == 0 {
			row(c, "none", func() {
				ui.Text(c, "词库为空。添加专有名词、产品名或人名，提升识别与润色的准确度。").FontSize(13).TextColor(colMuted)
			})
		}
		for _, t := range v.Snapshot.Terms {
			row(c, t.NormalizedTerm, func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
					ui.Row(c).Gap(8).Children(func() {
						ui.Text(c, t.Term).FontSize(14).FontWeight(500).SingleLine()
						if t.Locked {
							ui.Icon(c, iconLock).Size(12, 12).TextColor(colOrange).Label("已锁定")
						}
					})
					ui.Text(c, fmt.Sprintf("%s · 权重 %d", sourceLabel(t.Source), t.Weight)).FontSize(11.5).TextColor(colMuted).FontFeatures("tnum")
				})
				if smallPill(c, "edit", "编辑", pillGhost, nil, false) {
					v.term = t.Term
					v.editing = t.Term
					v.locked = t.Locked
				}
				if smallPill(c, "delete", "删除", pillDanger, nil, false) {
					v.Actions.Delete(t.Term)
				}
			})
		}
	})
}

func sourceLabel(s string) string {
	switch s {
	case "manual":
		return "手动"
	case "correction":
		return "纠错"
	case "ai":
		return "AI"
	case "imported":
		return "导入"
	}
	return s
}

func (v *View) refinement(c *ui.Context) {
	group(c, "engine", "润色引擎", nil, func() {
		row(c, "engine", func() {
			label(c, "引擎", "默认 Apple 本地模型；失败不会自动切换到远端。")
			if i := indexOf(engines, v.Config.Engine); segmented(c, "engine-seg", &i, engineLabels) {
				v.Config.Engine = engines[i]
			}
		})
		row(c, "mode", func() {
			label(c, "模式", "失败、超时或忠实度过低时保留 ASR 原文。")
			if i := indexOf(modes, v.Config.Mode); segmented(c, "mode-seg", &i, modeLabels) {
				v.Config.Mode = modes[i]
			}
		})
	})
	if v.Config.Engine == "remote" {
		notice(c, "remote", "显式远端模式：最终转写和词库将发送到所填 HTTPS 端点；音频不发送。", true)
	}
	group(c, "endpoint", "OpenAI 兼容接口", nil, func() {
		field(c, "接口", func() {
			i := indexOf(apis, v.Config.API)
			if segmented(c, "api-seg", &i, apiLabels) {
				v.Config.API = apis[i]
			}
			ui.Text(c, "POST 端点 + "+apiPaths[i]).FontSize(12).TextColor(colMuted).SingleLine().Grow(1).MinWidth(0)
		})
		field(c, "端点", func() {
			ui.TextInput(c, &v.Config.Endpoint).Placeholder("https://provider.example/v1").Grow(1).Label("端点")
		})
		field(c, "模型", func() { ui.TextInput(c, &v.Config.Model).Placeholder("模型名称").Grow(1).Label("模型") })
		field(c, "API key", func() {
			ui.TextInput(c, &v.secret).Password().Placeholder("仅保存到 macOS 钥匙串").Grow(1).Label("API key")
		})
	})
	ui.Row(c).Gap(12).Wrap().GapY(12).Children(func() {
		if pill(c, "delete-cred", "删除凭据", pillDanger, nil, nil, false) {
			v.Actions.DeleteCredential(v.Config.Endpoint)
		}
		ui.Box(c).Grow(1).MinWidth(24).Height(1).Background(colRule)
		if pill(c, "save-cred", "保存到钥匙串", pillSecondary, iconKey, nil, false) {
			v.Actions.Credential(v.Config.Endpoint, v.secret)
			v.secret = ""
		}
		if pill(c, "save-config", "保存设置", pillPrimary, nil, iconArrow, false) {
			v.Actions.Configure(v.Config)
		}
	})
}

func label(c *ui.Context, title, detail string) {
	ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
		ui.Text(c, title).FontSize(14).FontWeight(500)
		ui.Text(c, detail).FontSize(12).TextColor(colMuted)
	})
}

func field(c *ui.Context, name string, input func()) {
	row(c, name, func() {
		ui.Text(c, name).FontSize(13).TextColor(colSubtle).Width(72).Shrink(0)
		input()
	})
}

// segmented is a capsule segmented control: a black track whose chosen
// segment is the reference's white primary capsule.
func segmented(c *ui.Context, key string, selected *int, labels []string) bool {
	seg := ui.SegmentedBase(c.Key(key), selected, len(labels))
	seg.Track.Padding(3).Gap(2).Radius(999).Background(colWell).Border(1, colHairline).Shrink(0).Children(func() {
		for i, l := range labels {
			s := seg.Segment(i).Height(28).PaddingX(14).Radius(999).Label(l)
			switch {
			case i == *selected:
				s.Background(ui.Hex("#FFFFFF")).TextColor(colCanvas)
			case s.Hovered():
				s.Background(colPill).TextColor(colText)
			default:
				s.TextColor(colMuted)
			}
			s.Children(func() { ui.Text(c, l).FontSize(12.5).FontWeight(500).SingleLine() })
		}
	})
	return seg.Track.Changed()
}

func indexOf(list []string, v string) int {
	for i, s := range list {
		if s == v {
			return i
		}
	}
	return 0
}

func fallback(s, alt string) string {
	if s == "" {
		return alt
	}
	return s
}

func pick2(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
