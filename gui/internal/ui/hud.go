package ui

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// HUD window metrics: the capsule sits at the bottom centre, leaving room
// above it for the cancel button's tooltip.
const (
	HUDWidth  = 680
	HUDHeight = 128
	hudTail   = 26 // runes of transcript the capsule keeps, newest last
	miniBars  = 11
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
	const pitch, bar = 4.5, 2.5
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
			h = bar + a*10
			color = colOrangeDeep.Mix(colOrange, a)
		}
		x := x0 + float32(i)*pitch
		p.Fill(ui.Rect{X: x, Y: cy - h/2, W: bar, H: h}, color, bar/2)
	}
}

// hudLine is what the capsule says in each phase.
func hudLine(s Snapshot) (string, bool) {
	switch s.Phase {
	case "starting":
		return "正在准备…", false
	case "recording", "tail":
		if s.Partial == "" {
			return "请说话…", false
		}
		return s.Partial, true
	case "finalizing":
		if s.Partial == "" {
			return "正在识别…", false
		}
		return s.Partial, true
	case "refining":
		return s.Original, true
	}
	if s.Status != "" {
		return s.Status, false
	}
	return "等待语音", false
}

// DrawHUD renders the floating dictation capsule. It shares the snapshot of
// the main window; the app shows its panel without taking the keyboard.
func (v *View) DrawHUD(c *ui.Context) {
	c.SetTheme(theme)
	c.Root().Background(ui.Transparent)
	s := v.Snapshot
	ui.Box(c).Fill().Children(func() {
		capsule := ui.Row(c).Attach(ui.AnchorBottom, ui.AnchorBottom).Bottom(14).Height(52).Padding(7).Gap(10).
			Radius(26).Background(ui.Hex("#121212")).Border(1, ui.RGBA(255, 255, 255, 0.12)).
			Shadow(0, 10, 30, 0, ui.RGBA(0, 0, 0, 0.45))
		capsule.Children(func() {
			cancel := ui.ButtonBase(c.Key("hud-cancel")).Size(38, 38).Radius(19).Center().Label("取消").Tooltip("取消  Esc").
				Border(1, ui.RGBA(255, 255, 255, 0.16)).TextColor(colSubtle)
			cancel.Background(pick(cancel.Hovered(), colPillHover, colPill))
			cancel.Children(func() { ui.Icon(c, iconClose).Size(15, 15) })
			if cancel.Clicked() && s.SpeechActive {
				v.Actions.Cancel()
			}
			phase := s.Phase
			bins := s.Meter.Bins
			ui.Box(c).Size(54, 26).Draw(func(p *ui.Painter, r ui.Rect) { drawMini(p, r, bins, phase) })
			line, spoken := hudLine(s)
			ui.Text(c, tailText(line, hudTail)).FontSize(15).FontWeight(500).SingleLine().MaxWidth(470).
				TextColor(pick(spoken, colText, colMuted)).PaddingX(2)
			commit := ui.ButtonBase(c.Key("hud-commit")).Size(38, 38).Radius(19).Center().Label("立即插入原文").
				Tooltip("跳过 AI 润色，立即插入").Disabled(!s.CanCommit)
			if s.CanCommit {
				commit.Background(pick(commit.Hovered(), ui.Hex("#EBEBEF"), ui.Hex("#FFFFFF"))).TextColor(colCanvas)
			} else {
				commit.Background(colPill).TextColor(colFaint)
			}
			commit.Children(func() { ui.Icon(c, iconCheck).Size(16, 16) })
			if commit.Clicked() && s.CanCommit {
				v.Actions.Commit()
			}
		})
	})
}
