package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestTailTextKeepsNewestWords(t *testing.T) {
	if tailText("短句", 5) != "短句" {
		t.Fatal("short text changed")
	}
	if got := tailText("一二三四五六七", 3); got != "…五六七" {
		t.Fatalf("tail %q", got)
	}
}

func TestHUDLineFollowsPhase(t *testing.T) {
	cases := []struct {
		s      Snapshot
		line   string
		spoken bool
	}{
		{Snapshot{Phase: "recording"}, "请说话…", false},
		{Snapshot{Phase: "recording", Partial: "你好"}, "你好", true},
		{Snapshot{Phase: "refining", Partial: "你好", Original: "你好。"}, "你好。", true},
		{Snapshot{Phase: "ready", Status: "已取消"}, "已取消", false},
	}
	for _, c := range cases {
		if line, spoken := hudLine(c.s); line != c.line || spoken != c.spoken {
			t.Fatalf("%+v: %q %v", c.s, line, spoken)
		}
	}
}

func TestHUDCommitOnlyWhenAllowed(t *testing.T) {
	var rec intents
	commits := 0
	v := testView(&rec)
	v.Actions.Commit = func() { commits++ }
	v.Snapshot.SpeechActive, v.Snapshot.Phase, v.Snapshot.Partial = true, "recording", "实现波形的"
	tt := ui.NewTester(v.DrawHUD, HUDWidth, HUDHeight)
	_ = tt.Click("立即插入原文")
	if commits != 0 {
		t.Fatal("commit while recording")
	}
	if err := tt.Click("取消"); err != nil || rec.cancel != 1 {
		t.Fatalf("cancel: %v", err)
	}
	v.Snapshot.Phase, v.Snapshot.Original, v.Snapshot.CanCommit = "refining", "实现波形的，能小小文字的？", true
	tt.Frame()
	if err := tt.Click("立即插入原文"); err != nil || commits != 1 {
		t.Fatalf("commit: %v", err)
	}
	if dir := os.Getenv("GOVOICE_UI_SNAPSHOTS"); dir != "" {
		v.Snapshot.Phase = "recording"
		for i := range v.Snapshot.Meter.Bins {
			v.Snapshot.Meter.Bins[i] = []float64{0.05, 0.3, 0.12, 0.5, 0.2}[i%5]
		}
		tt.SetScale(2)
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, "hud.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}
