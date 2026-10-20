package app

import "github.com/vfasky/my-ai-audio/gui/internal/wordlists"

func (a *application) configurePacks(packs wordlists.Config) {
	previous := a.config.Packs
	a.config.Packs = packs
	if err := a.writeSettings(); err != nil {
		a.config.Packs = previous
		a.view.Snapshot.PacksFeedback = "保存词库设置失败，请重试。"
		a.view.Snapshot.PacksFeedbackOK = false
	} else {
		a.view.Snapshot.PacksFeedback = "词库设置已保存，下次语音生效。"
		a.view.Snapshot.PacksFeedbackOK = true
	}
	a.view.Snapshot.PacksFeedbackFor = packs
	a.refresh()
}
