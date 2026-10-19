package vocabulary

import (
	"os"
	"path/filepath"
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
	if top := s.Top(1); len(top) != 1 || top[0] != "MYGO" || s.Terms[0].LastUsedAt.IsZero() {
		t.Fatal(top)
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
