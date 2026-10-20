package vocabulary

import (
	"fmt"
	"slices"
	"testing"
)

func TestCorrectionTermsIncludeAllTargetsWithoutChangingTop100(t *testing.T) {
	s := Store{}
	for i := 0; i < 120; i++ {
		if err := s.Put(Term{Term: fmt.Sprintf("term%d", i), Source: "manual", Weight: 50}, false); err != nil {
			t.Fatal(err)
		}
	}
	_ = s.Put(Term{Term: "伊智", Source: "manual", Weight: 1, Locked: true}, false)
	_ = s.PutReplacement("", Replacement{From: "施玉宝", To: "私域宝"})
	_ = s.PutReplacement("", Replacement{From: "私欲宝", To: "私域宝"})
	_ = s.PutReplacement("", Replacement{From: "够我四", To: "Go Voice"})
	all, hints := s.CorrectionTerms(), s.Contextual(100)
	if len(hints) != 100 || hints[0] != "伊智" || slices.Contains(hints, "私域宝") || len(all) != 123 || all[0] != "伊智" {
		t.Fatalf("hints=%v all=%v", hints, all)
	}
	for _, bad := range []string{"施玉宝", "私欲宝", "够我四"} {
		if slices.Contains(all, bad) {
			t.Fatal("wrong spelling indexed", bad)
		}
	}
	all[0] = "changed"
	if s.Terms[120].Term != "伊智" {
		t.Fatal("correction terms alias store")
	}
}

func TestSpacedCorrectionProtectsAWordOutsideRecognitionHints(t *testing.T) {
	s := Store{}
	for i := 0; i < 100; i++ {
		_ = s.Put(Term{Term: fmt.Sprintf("term%d", i), Source: "manual", Weight: 100}, false)
	}
	_ = s.Put(Term{Term: "幻羽心", Source: "manual", Weight: 1}, false)
	_ = s.PutReplacement("", Replacement{From: "心一", To: "心仪"})
	if got := s.CorrectSegmented("幻羽心 一"); got != "幻羽心 一" {
		t.Fatalf("cut a known word outside Top 100: %s", got)
	}
}
