package vocabulary

import (
	"fmt"
	"slices"
	"testing"
)

func TestContextualIncludesOnlyUniqueTargetsWithinLimit(t *testing.T) {
	s := Store{}
	_ = s.Put(Term{Term: "伊智", Source: "manual", Weight: 50, Locked: true}, false)
	for _, r := range []Replacement{{From: "施玉宝", To: "私域宝"}, {From: "私欲宝", To: "私域宝"}, {From: "一直", To: "伊智"}, {From: "够我四", To: "Go Voice"}} {
		if err := s.PutReplacement("", r); err != nil {
			t.Fatal(err)
		}
	}
	if got := s.Contextual(100); !slices.Equal(got, []string{"伊智", "私域宝", "Go Voice"}) {
		t.Fatal(got)
	}
	if got := s.Contextual(2); !slices.Equal(got, []string{"伊智", "私域宝"}) {
		t.Fatal(got)
	}
	if len(s.Contextual(0)) != 0 || len(s.Contextual(-1)) != 0 {
		t.Fatal("bad limit")
	}
	if len(s.Terms) != 1 {
		t.Fatal("derived targets persisted as terms")
	}
	s.DeleteReplacement("施玉宝")
	s.DeleteReplacement("私欲宝")
	if slices.Contains(s.Contextual(100), "私域宝") {
		t.Fatal("deleted target retained")
	}
	for i := 0; i < 120; i++ {
		_ = s.Put(Term{Term: fmt.Sprintf("term%d", i), Source: "manual", Weight: 50}, false)
	}
	if got := s.Contextual(200); len(got) != 100 || got[0] != "伊智" || slices.Contains(got, "Go Voice") {
		t.Fatalf("priority/limit: %v", got)
	}
}

func TestSegmentedReplacementsRespectWholeTokensAndSeparators(t *testing.T) {
	s := Store{Replacements: []Replacement{
		{From: "施玉宝", To: "私域宝"},
		{From: "心一", To: "心仪"},
		{From: "玉宝", To: "域宝"},
	}}
	for _, tc := range []struct{ in, want string }{
		{"🙂打开施 玉 宝后台", "🙂打开私域宝后台"},
		{"施\t玉\u3000宝上线了", "私域宝上线了"},
		{"施玉宝上线了", "私域宝上线了"},
		{"施，玉宝", "施，域宝"}, // The literal rule still applies; punctuation is not crossed.
		{"施\n玉 宝", "施\n域宝"},
		{"施\r玉 宝", "施\r域宝"},
		{"施\u2028玉 宝", "施\u2028域宝"},
		{"一心 一意完成工作", "一心 一意完成工作"},
		{"施 x 玉 宝", "施 x 域宝"},
		{"施 玉 宝和施 玉 宝", "私域宝和私域宝"},
	} {
		if got := s.CorrectSegmented(tc.in); got != tc.want {
			t.Errorf("%q -> %q, want %q", tc.in, got, tc.want)
		}
	}
	if got := s.Correct("施 玉 宝"); got != "施 玉 宝" {
		t.Fatalf("live literal path changed: %q", got)
	}
}

func TestSegmentedRulesPreferLongerAndKeepChainingBounded(t *testing.T) {
	s := Store{Replacements: []Replacement{{From: "施玉", To: "石玉"}, {From: "施玉宝", To: "私域宝"}, {From: "公寓", To: "公域"}, {From: "公域托客", To: "公域拓客"}, {From: "甲乙", To: "甲 乙"}}}
	if got := s.CorrectSegmented("施 玉 宝，公 寓托客，甲 乙"); got != "私域宝，公域拓客，甲 乙" {
		t.Fatal(got)
	}
}

func TestRepeatedCorrectionDoesNotExpandItsOwnTargets(t *testing.T) {
	s := Store{Replacements: []Replacement{{From: "go", To: "Go Voice"}, {From: "施玉宝", To: "私域宝"}}}
	preview := s.Correct("go 施 玉 宝")
	final := s.CorrectSegmented(preview)
	if final != "Go Voice 私域宝" || s.CorrectSegmented(final) != final {
		t.Fatalf("repeated expansion: %q -> %q", preview, final)
	}
	s.Replacements = append(s.Replacements, Replacement{From: "宝", To: "私域宝"})
	if got := s.Correct("Go Voice 私域宝"); got != "Go Voice 私域宝" {
		t.Fatal(got)
	}
	// A rule explicitly targeting the whole previous output still wins.
	s.Replacements = append(s.Replacements, Replacement{From: "Go Voice", To: "语音助手"})
	if got := s.Correct("Go Voice"); got != "语音助手" {
		t.Fatal(got)
	}
}

func TestSegmentationRespectsKnownNamesAndSnapshotEdits(t *testing.T) {
	s := Store{Replacements: []Replacement{{From: "玉宝", To: "域宝"}}}
	_ = s.Put(Term{Term: "幻玉", Source: "manual", Weight: 50}, false)
	snapshot := s.Snapshot()
	// The custom name must not be cut in half to match the spaced source.
	if got := snapshot.CorrectSegmented("幻玉 宝"); got != "幻玉 宝" {
		t.Fatal(got)
	}
	_ = s.PutReplacement("玉宝", Replacement{From: "玉宝", To: "新词"})
	_ = s.Update("幻玉", Term{Term: "别名", Source: "manual", Weight: 50})
	if got := snapshot.CorrectSegmented("玉 宝"); got != "域宝" {
		t.Fatalf("snapshot mutated: %q", got)
	}
	if got := s.CorrectSegmented("玉 宝"); got != "新词" {
		t.Fatalf("new target ignored: %q", got)
	}
}
