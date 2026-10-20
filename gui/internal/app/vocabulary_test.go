package app

import (
	"slices"
	"testing"

	"github.com/vfasky/my-ai-audio/gui/internal/device"
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
