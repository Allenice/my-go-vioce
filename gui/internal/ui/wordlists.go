package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/vfasky/my-ai-audio/gui/internal/wordlists"
)

func publicSummary(config wordlists.Config) string {
	var active []string
	for _, source := range wordlists.Catalog() {
		if config.Enabled(source.ID) {
			name := "通用词库"
			if source.ID == "it" {
				name = "IT 行业包"
			}
			active = append(active, fmt.Sprintf("%s %d 词", name, source.Terms))
		}
	}
	if len(active) == 0 {
		return "公共词库已关闭，仍可使用个人词条和替换规则。"
	}
	return "已启用：" + strings.Join(active, " · ") + "。在语音结束后提供纠错候选。"
}

func (v *View) wordlists(c *ui.Context) {
	notice(c, "packs-about", "公共词库为常用词和专业术语提供纠错候选，需要开启 AI 润色或 Jev 判断。个人词条和显式替换规则继续优先；公共词库不会填入你的个人词条，也不会占用识别提示的 Top 100 名额。", false)
	group(c, "public-wordlists", "本地公共词库", nil, func() {
		for _, source := range wordlists.Catalog() {
			row(c, "pack-"+source.ID, func() {
				name, detail, toggle := "通用基础词库", "Jieba 常用中文词；随应用提供，离线检索。", "启用通用基础词库"
				disabled := &v.Packs.BaseDisabled
				if source.ID == "it" {
					name, detail, toggle = "IT 行业包", "THUOCL：编程、数据库、网络等中文术语。", "启用 IT 行业包"
					disabled = &v.Packs.ITDisabled
				}
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Text(c, fmt.Sprintf("%s · %d 词", name, source.Terms)).FontSize(14).FontWeight(500)
					ui.Text(c, detail).FontSize(12).TextColor(colMuted).LineHeight(1.4)
				})
				enabled := !*disabled
				ui.Switch(c, &enabled).Label(toggle)
				*disabled = !enabled
			})
		}
	})
	notice(c, "packs-private", "词库数据随应用提供，不会在使用时下载。选择远端润色或开启 Jev 后，只有本句命中的候选和必要上下文会随原流程发送，不发送完整公共词库。", false)
	if pill(c, "save-packs", "保存词库设置", pillPrimary, nil, iconArrow, false) {
		v.Actions.ConfigurePacks(v.Packs)
	}
	s := v.Snapshot
	if s.PacksFeedback != "" && s.PacksFeedbackFor == v.Packs {
		feedback(c, s.PacksFeedback, s.PacksFeedbackOK)
	}
}
