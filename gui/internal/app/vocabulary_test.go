package app

import (
	"fmt"
	"slices"
	"testing"

	"github.com/vfasky/my-ai-audio/gui/internal/device"
	"github.com/vfasky/my-ai-audio/gui/internal/refinement"
	"github.com/vfasky/my-ai-audio/gui/internal/vocabulary"
)

func TestSessionVocabularyIncludesReplacementTargets(t *testing.T) {
	a := &application{}
	_ = a.store.Put(vocabulary.Term{Term: "伊智", Source: "manual", Weight: 50}, false)
	_ = a.store.PutReplacement("", vocabulary.Replacement{From: "施玉宝", To: "私域宝"})
	a.effects([]device.Effect{{Type: "start", ID: 1}})
	if !slices.Equal(a.terms, []string{"伊智", "私域宝"}) {
		t.Fatal(a.terms)
	}
	_ = a.store.PutReplacement("施玉宝", vocabulary.Replacement{From: "施玉宝", To: "私域助手"})
	if !slices.Equal(a.terms, []string{"伊智", "私域宝"}) {
		t.Fatal("active session vocabulary changed", a.terms)
	}
	a.effects([]device.Effect{{Type: "start", ID: 2}})
	if !slices.Equal(a.terms, []string{"伊智", "私域助手"}) {
		t.Fatal("new session missed the target edit", a.terms)
	}
}

func TestSessionRetrievesBeyondRecognitionLimitAndFreezesVocabulary(t *testing.T) {
	a := &application{}
	for i := 0; i < 120; i++ {
		_ = a.store.Put(vocabulary.Term{Term: fmt.Sprintf("测试条目%d", i), Source: "manual", Weight: 100}, false)
	}
	_ = a.store.Put(vocabulary.Term{Term: "伊智", Source: "manual", Weight: 1}, false)
	_ = a.store.Put(vocabulary.Term{Term: "一致性", Source: "manual", Weight: 1}, false)
	_ = a.store.PutReplacement("", vocabulary.Replacement{From: "施玉宝", To: "私域宝"})
	a.effects([]device.Effect{{Type: "start", ID: 1}})
	if len(a.terms) != 100 || slices.Contains(a.terms, "伊智") || slices.Contains(a.terms, "私域宝") {
		t.Fatal("recognition budget changed", a.terms)
	}
	text := "保持一致性，联系一致科技，再打开私欲宝"
	got, err := refinement.NewLexicon(a.correctionTerms).Retrieve(text, a.terms)
	for _, term := range []string{"伊智", "一致性", "私域宝"} {
		if err != nil || !slices.Contains(got, term) {
			t.Fatalf("missing %s: %v %v", term, got, err)
		}
	}
	_ = a.store.Delete("伊智", false)
	_ = a.store.PutReplacement("施玉宝", vocabulary.Replacement{From: "施玉宝", To: "私域助手"})
	if !slices.Contains(a.correctionTerms, "伊智") || !slices.Contains(a.correctionTerms, "私域宝") {
		t.Fatal("editing changed an active session")
	}
	a.effects([]device.Effect{{Type: "start", ID: 2}})
	if slices.Contains(a.correctionTerms, "伊智") || slices.Contains(a.correctionTerms, "私域宝") || !slices.Contains(a.correctionTerms, "私域助手") {
		t.Fatal("new session retained old vocabulary", a.correctionTerms)
	}
}
