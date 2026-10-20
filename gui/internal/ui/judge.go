package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/vfasky/my-ai-audio/gui/internal/jev"
)

// jevPresets are providers of Jev the tab fills in with one click; any
// other System One endpoint can be typed.
var jevPresets = []struct {
	key, label string
	config     jev.Config
}{
	{"jev-typesafe", "TypeSafe 官方", jev.Config{Endpoint: jev.DefaultEndpoint, Model: jev.DefaultModel}},
	{"jev-maas", "支付宝 MaaS", jev.Config{Endpoint: "https://maas-api.antdigital.com/v1/systemone", Model: "jev-1.13"}},
}

// judge is the Jev tab of the vocabulary page: Jev decides, with a
// probability, whether a span that sounds like a term is the term. It
// runs at TypeSafe or at any provider of its System One API.
func (v *View) judge(c *ui.Context) {
	s := v.Snapshot
	notice(c, "jev-about", fmt.Sprintf("读音像词库词的片段（如“一致”像“伊智”）不再全靠 AI 润色决定：Jev 对每处给出“说的就是这个词”的概率，≥ %.0f%% 直接替换，≤ %.0f%% 保留原文并让 AI 润色不要改，其余仍由 AI 润色判断。只处理两个字以上的中文词条；英文词条和替换词不受影响。", jev.Replace*100, jev.Keep*100), false)
	group(c, "jev", "Jev 词库判断", nil, func() {
		row(c, "jev-enabled", func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, "用 Jev 判断同音词替换").FontSize(14).FontWeight(500).SingleLine()
				ui.Text(c, fmt.Sprintf("每次语音结束、AI 润色之前运行；关闭 AI 润色时也生效。超过 %d 秒或失败时跳过，按原流程处理。", int(jev.Timeout.Seconds()))).
					FontSize(12).TextColor(colMuted).LineHeight(1.4)
			})
			ui.Switch(c, &v.Jev.Enabled).Label("启用 Jev 词库判断")
		})
		row(c, "jev-presets", func() {
			label(c, "服务商", "兼容 TypeSafe System One 接口（POST /v1/systemone）的服务都可以填写。")
			current := jev.Normalize(v.Jev)
			for _, p := range jevPresets {
				on := current.Endpoint == p.config.Endpoint && current.Model == p.config.Model
				if smallPill(c, p.key, p.label, pickPill(on, pillPrimary, pillSecondary), nil, false) {
					v.Jev.Endpoint, v.Jev.Model = p.config.Endpoint, p.config.Model
				}
			}
		})
		field(c, "接口地址", func() {
			ui.TextInput(c, &v.Jev.Endpoint).Placeholder(jev.DefaultEndpoint).Grow(1).Label("Jev 接口地址")
		})
		field(c, "模型", func() {
			ui.TextInput(c, &v.Jev.Model).Placeholder(jev.DefaultModel).Grow(1).Label("Jev 模型")
		})
		field(c, "API key", func() {
			ui.TextInput(c, &v.jevSecret).Password().Placeholder("按接口地址分别保存到 macOS 钥匙串").Grow(1).Label("Jev API key")
		})
	})
	if v.Jev.Enabled {
		notice(c, "jev-privacy", "开启后，识别文字中命中词库读音的片段及其前后各约 40 个字，连同命中的词条，会发送到 "+v.Jev.Host()+" 判断。不发送音频和完整词库；没有命中时不发送任何内容。", true)
	}
	ui.Row(c).Gap(12).Wrap().GapY(12).Children(func() {
		if pill(c, "jev-delete-key", "删除凭据", pillDanger, nil, nil, false) {
			v.Actions.DeleteJevKey()
		}
		ui.Box(c).Grow(1).MinWidth(24).Height(1).Background(colRule)
		if pill(c, "jev-test", pickText(s.JevTesting, "正在测试…", "测试连接"), pillSecondary, nil, nil, s.JevTesting) {
			v.Actions.TestJev(v.Jev, v.jevSecret)
		}
		if pill(c, "jev-save", "保存设置", pillPrimary, nil, iconArrow, false) {
			v.Actions.ConfigureJev(v.Jev, v.jevSecret)
			v.jevSecret = ""
		}
	})
	if s.JevFeedback != "" && !s.JevTesting && jev.Normalize(s.JevFeedbackFor) == jev.Normalize(v.Jev) {
		feedback(c, s.JevFeedback, s.JevFeedbackOK)
	}
}

func pickPill(cond bool, a, b pillKind) pillKind {
	if cond {
		return a
	}
	return b
}
