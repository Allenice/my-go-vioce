package ui

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// HUD window metrics: the capsule sits at the bottom centre, with the
// preview of the text above it and room for the cancel button's tooltip.
const (
	HUDWidth  = 510
	HUDHeight = 172
	// previewTail is the runes of transcript the preview keeps, newest
	// last: about three lines of Chinese.
	previewTail = 90
	miniBars    = 11
)

// tailText keeps the newest max runes, as a dictation bar shows the words
// being spoken rather than the first ones.
func tailText(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return "…" + string(r[len(r)-max:])
}

// miniLevels samples the mirrored waveform for the capsule's short bars.
func miniLevels(bins [32]float64) [miniBars]float32 {
	full := waveLevels(bins)
	var out [miniBars]float32
	mid, step := waveBars/2, 3
	for i := range out {
		out[i] = full[mid+(i-miniBars/2)*step]
	}
	return out
}

func drawMini(p *ui.Painter, r ui.Rect, bins [32]float64, phase string) {
	const pitch, bar = 3.375, 1.875
	x0 := r.X + (r.W-(miniBars-1)*pitch-bar)/2
	cy := r.Y + r.H/2
	live, working := isLive(phase), isWorking(phase) || phase == "finalizing"
	levels := miniLevels(bins)
	head := float32(-100)
	if working {
		p.AnimationFrame()
		const period = 1100 * time.Millisecond
		head = float32(p.Now().UnixNano()%int64(period))/float32(period)*(miniBars+4) - 2
	}
	for i := 0; i < miniBars; i++ {
		h, color := float32(bar), colIdleDot
		switch {
		case live:
			h = max(bar, levels[i]*r.H)
			color = colOrangeDeep.Mix(colOrange, 0.45+0.55*levels[i])
		case working:
			a := max(0, 1-abs32(float32(i)-head)/2.5)
			h = bar + a*7.5
			color = colOrangeDeep.Mix(colOrange, a)
		}
		x := x0 + float32(i)*pitch
		p.Fill(ui.Rect{X: x, Y: cy - h/2, W: bar, H: h}, color, bar/2)
	}
}

// hudLine is what the capsule says in each phase; the text itself shows in
// the preview above it.
func hudLine(s Snapshot) string {
	switch s.Phase {
	case "starting":
		return "正在准备…"
	case "recording", "tail":
		if s.Partial == "" {
			return "请说话…"
		}
		return "正在聆听…"
	case "finalizing":
		return "正在识别…"
	case "refining":
		return "AI 润色中…  可直接插入原文"
	}
	if s.Status != "" {
		return s.Status
	}
	return "等待语音"
}

// hudPreview is the text the preview above the capsule shows: what was
// recognized so far, then while refinement runs the ASR original that the
// confirm button would insert.
func hudPreview(s Snapshot) string {
	switch s.Phase {
	case "recording", "tail", "finalizing":
		return s.Partial
	case "refining":
		return s.Original
	}
	return ""
}

// DrawHUD renders the floating dictation capsule. It shares the snapshot of
// the main window; the app shows its panel without taking the keyboard.
func (v *View) DrawHUD(c *ui.Context) {
	c.SetTheme(theme)
	c.Root().Background(ui.Transparent)
	s := v.Snapshot
	ui.Box(c).Fill().Children(func() {
		if text := hudPreview(s); text != "" {
			ui.Box(c).Attach(ui.AnchorBottom, ui.AnchorBottom).Bottom(57).MaxWidth(HUDWidth-30).Padding(9, 15).
				Radius(16).Background(ui.Hex("#121212")).Border(1, ui.RGBA(255, 255, 255, 0.12)).
				Shadow(0, 7.5, 22.5, 0, ui.RGBA(0, 0, 0, 0.45)).Children(func() {
				ui.Text(c, tailText(text, previewTail)).FontSize(13.5).LineHeight(1.45).MaxLines(3).TextColor(colText)
			})
		}
		capsule := ui.Row(c).Attach(ui.AnchorBottom, ui.AnchorBottom).Bottom(10.5).Height(39).Padding(5.25).Gap(7.5).
			Radius(19.5).Background(ui.Hex("#121212")).Border(1, ui.RGBA(255, 255, 255, 0.12)).
			Shadow(0, 7.5, 22.5, 0, ui.RGBA(0, 0, 0, 0.45))
		capsule.Children(func() {
			cancel := ui.ButtonBase(c.Key("hud-cancel")).Size(28.5, 28.5).Radius(14.25).Center().Label("取消").Tooltip("取消  Esc").
				Border(1, ui.RGBA(255, 255, 255, 0.16)).TextColor(colSubtle)
			cancel.Background(pick(cancel.Hovered(), colPillHover, colPill))
			cancel.Children(func() { ui.Icon(c, iconClose).Size(11.25, 11.25) })
			if cancel.Clicked() && s.SpeechActive {
				v.Actions.Cancel()
			}
			phase := s.Phase
			bins := s.Meter.Bins
			ui.Box(c).Size(40.5, 19.5).Draw(func(p *ui.Painter, r ui.Rect) { drawMini(p, r, bins, phase) })
			ui.Text(c, hudLine(s)).FontSize(11.25).FontWeight(500).SingleLine().MaxWidth(352.5).
				TextColor(colMuted).PaddingX(1.5)
			commit := ui.ButtonBase(c.Key("hud-commit")).Size(28.5, 28.5).Radius(14.25).Center().Label("立即插入原文").
				Tooltip("跳过 AI 润色，立即插入").Disabled(!s.CanCommit)
			if s.CanCommit {
				commit.Background(pick(commit.Hovered(), ui.Hex("#EBEBEF"), ui.Hex("#FFFFFF"))).TextColor(colCanvas)
			} else {
				commit.Background(colPill).TextColor(colFaint)
			}
			commit.Children(func() { ui.Icon(c, iconCheck).Size(12, 12) })
			if commit.Clicked() && s.CanCommit {
				v.Actions.Commit()
			}
		})
	})
}
