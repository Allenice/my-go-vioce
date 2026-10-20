package vocabulary

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeduplicateLockedAndPersistence(t *testing.T) {
	s := Store{}
	if e := s.Put(Term{Term: " MyGo ", Source: "manual", Locked: true, Weight: 90}, false); e != nil {
		t.Fatal(e)
	}
	if e := s.Put(Term{Term: "mygo", Source: "ai", Weight: 20}, true); e == nil {
		t.Fatal("AI replaced lock")
	}
	if e := s.Delete("MYGO", true); e == nil {
		t.Fatal("AI deleted lock")
	}
	if e := s.Put(Term{Term: "MYGO", Source: "manual", Locked: true, Weight: 80}, false); e != nil || len(s.Terms) != 1 {
		t.Fatal(e, s.Terms)
	}
	_ = s.Put(Term{Term: "伊智", Source: "correction", Weight: 95}, false)
	if top := s.Top(1); len(top) != 1 || top[0] != "MYGO" {
		t.Fatal(top)
	}
	if !s.Terms[0].LastUsedAt.IsZero() {
		t.Fatal("Top must not rewrite lastUsedAt")
	}
	path := filepath.Join(t.TempDir(), "vocabulary.json")
	if e := s.Save(path); e != nil {
		t.Fatal(e)
	}
	loaded, e := Load(path)
	if e != nil || len(loaded.Terms) != 2 {
		t.Fatal(e)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
}

func TestReplacementsCorrectRecognition(t *testing.T) {
	s := Store{}
	for _, r := range []Replacement{{From: " 施玉宝 ", To: "私域宝"}, {From: "私语宝", To: "私域宝"}, {From: "go", To: "Go Voice"}, {From: "够我四", To: "Go Voice"}, {From: "go boys", To: "Go Voice"}} {
		if e := s.PutReplacement("", r); e != nil {
			t.Fatal(r, e)
		}
	}
	if s.Replacements[0].From != "施玉宝" || s.Replacements[0].CreatedAt.IsZero() {
		t.Fatalf("trimmed %+v", s.Replacements[0])
	}
	for in, want := range map[string]string{
		"我们的施玉宝和私语宝都上线了": "我们的私域宝和私域宝都上线了",
		"打开够我四":              "打开Go Voice",
		"GO BOYS 发布了":        "Go Voice 发布了",
		"go 一下":              "Go Voice 一下",
		"google 和 gopher 不变": "google 和 gopher 不变",
		"没有替换词的句子":           "没有替换词的句子",
	} {
		if got := s.Correct(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

func TestReplacementRules(t *testing.T) {
	s := Store{}
	_ = s.Put(Term{Term: "伊智", Source: "manual", Weight: 50}, false)
	_ = s.PutReplacement("", Replacement{From: "施玉宝", To: "私域宝"})
	for r, want := range map[Replacement]error{
		{From: "施玉宝", To: "私域通"}:  ErrReplacementTaken,
		{From: "伊智", To: "一致"}:    ErrReplacementIsTerm,
		{From: "", To: "私域宝"}:     ErrInvalidReplacement,
		{From: "私域宝", To: ""}:     ErrInvalidReplacement,
		{From: "私域宝", To: " 私域宝"}: ErrInvalidReplacement,
	} {
		if e := s.PutReplacement("", r); !errors.Is(e, want) {
			t.Errorf("%+v: %v, want %v", r, e, want)
		}
	}
	// An edit changes the source and the target in place.
	created := s.Replacements[0].CreatedAt
	if e := s.PutReplacement("施玉宝", Replacement{From: "施域宝", To: "私域宝"}); e != nil || len(s.Replacements) != 1 || s.Replacements[0].CreatedAt != created {
		t.Fatalf("edit %+v %v", s.Replacements, e)
	}
	if e := s.PutReplacement("施域宝", Replacement{From: "施域宝", To: "私域宝 Pro"}); e != nil || s.Correct("施域宝") != "私域宝 Pro" {
		t.Fatalf("retarget %+v %v", s.Replacements, e)
	}
	s.DeleteReplacement("施域宝")
	if len(s.Replacements) != 0 {
		t.Fatal("not deleted")
	}
}

func TestReplacementsPersistAndMigrateAliases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vocabulary.json")
	s := Store{}
	_ = s.Put(Term{Term: "伊智", Source: "manual", Weight: 50}, false)
	_ = s.PutReplacement("", Replacement{From: "施玉宝", To: "私域宝"})
	if e := s.Save(path); e != nil {
		t.Fatal(e)
	}
	loaded, e := Load(path)
	if e != nil || loaded.Correct("施玉宝") != "私域宝" || len(loaded.Terms) != 1 {
		t.Fatalf("load %+v %v", loaded, e)
	}
	// Version 0.1 kept replacements as aliases of a term.
	old := `{"terms":[{"term":"私域宝","source":"manual","weight":50,"aliases":["施玉宝","私语宝"]},{"term":"伊智","source":"manual","weight":50,"aliases":["伊智"]}]}`
	if e := os.WriteFile(path, []byte(old), 0o600); e != nil {
		t.Fatal(e)
	}
	loaded, e = Load(path)
	if e != nil || len(loaded.Replacements) != 2 || loaded.Correct("私语宝") != "私域宝" || loaded.Terms[0].Aliases != nil {
		t.Fatalf("migrate %+v %v", loaded, e)
	}
}

func TestReplacementsChain(t *testing.T) {
	s := Store{}
	for _, r := range []Replacement{{From: "公寓", To: "公域"}, {From: "公域托客", To: "公域拓客"}, {From: "思域", To: "私域"}, {From: "甲", To: "乙"}, {From: "乙", To: "甲"}} {
		if e := s.PutReplacement("", r); e != nil {
			t.Fatal(r, e)
		}
	}
	for in, want := range map[string]string{
		"公寓托客是思域的入口": "公域拓客是私域的入口",
		"公域托客":       "公域拓客",
	} {
		if got := s.Correct(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
	// Rules that undo each other apply once.
	if got := s.Correct("甲乙"); got != "乙甲" {
		t.Fatalf("cycle %q", got)
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	s := Store{}
	_ = s.Put(Term{Term: "私域宝", Source: "manual", Weight: 80, Locked: true}, false)
	_ = s.Put(Term{Term: "伊智", Source: "correction", Weight: 50}, false)
	_ = s.PutReplacement("", Replacement{From: "石玉宝", To: "私域宝"})
	b, err := s.Export()
	if err != nil || !strings.Contains(string(b), `"format": "go-voice-vocabulary"`) || strings.Contains(string(b), "lastUsedAt") {
		t.Fatalf("export %s %v", b, err)
	}
	other := Store{}
	res, err := other.Import(b)
	if err != nil || res != (ImportResult{TermsAdded: 2, ReplacementsAdded: 1}) {
		t.Fatalf("import %+v %v", res, err)
	}
	if other.Terms[0].Weight != 80 || !other.Terms[0].Locked || other.Terms[0].Source != "imported" || other.Correct("石玉宝") != "私域宝" {
		t.Fatalf("imported %+v", other)
	}
}

func TestImportMergesAndSkips(t *testing.T) {
	s := Store{}
	_ = s.Put(Term{Term: "伊智", Source: "manual", Weight: 50}, false)
	_ = s.PutReplacement("", Replacement{From: "石玉宝", To: "私域"})
	file := `{"terms":[{"term":"伊智","locked":true},{"term":"新词"},{"term":""}],
	"replacements":[{"from":"石玉宝","to":"私域宝"},{"from":"公寓","to":"公域"},{"from":"伊智","to":"一致"},{"from":"x","to":"x"}]}`
	res, err := s.Import([]byte(file))
	if err != nil || res != (ImportResult{TermsAdded: 1, TermsUpdated: 1, ReplacementsAdded: 1, ReplacementsUpdated: 1, Skipped: 3}) {
		t.Fatalf("merge %+v %v", res, err)
	}
	if !s.Terms[0].Locked || s.Terms[0].Source != "manual" || len(s.Terms) != 2 || s.Correct("石玉宝公寓") != "私域宝公域" {
		t.Fatalf("store %+v", s)
	}
	// The app's own vocabulary.json imports too.
	if _, err := (&Store{}).Import([]byte(`{"terms":[{"term":"a","normalizedTerm":"a","source":"manual","weight":50,"createdAt":"2026-10-20T00:00:00Z"}],"replacements":[{"from":"b","to":"c","createdAt":"2026-10-20T00:00:00Z"}]}`)); err != nil {
		t.Fatal(err)
	}
	// Other files change nothing.
	before := len(s.Terms)
	for _, bad := range []string{`not json`, `{"format":"other","terms":[{"term":"z"}]}`, `{}`, `[]`} {
		if _, err := s.Import([]byte(bad)); !errors.Is(err, ErrImportFormat) || len(s.Terms) != before {
			t.Fatalf("%s: %v", bad, err)
		}
	}
}
