package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/vfasky/my-ai-audio/gui/internal/device"
	"github.com/vfasky/my-ai-audio/gui/internal/wordlists"
)

func TestPacksDefaultEnabledAndChangesWaitForNextSession(t *testing.T) {
	a := &application{directory: t.TempDir()}
	a.effects([]device.Effect{{Type: "start", ID: 1}})
	if !a.sessionPacks.Enabled("base") || !a.sessionPacks.Enabled("it") || len(a.terms) != 0 || len(a.correctionTerms) != 0 {
		t.Fatal("public packs changed private hints or defaults")
	}
	a.configurePacks(wordlists.Config{ITDisabled: true})
	if !a.view.Snapshot.PacksFeedbackOK || !a.sessionPacks.Enabled("it") {
		t.Fatal("active session changed")
	}
	b, err := os.ReadFile(filepath.Join(a.directory, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved settings
	if json.Unmarshal(b, &saved) != nil || !saved.Packs.ITDisabled || saved.Packs.BaseDisabled {
		t.Fatalf("settings: %s", b)
	}
	if _, err := os.Stat(filepath.Join(a.directory, "vocabulary.json")); !os.IsNotExist(err) {
		t.Fatal("public settings wrote personal vocabulary")
	}
	a.effects([]device.Effect{{Type: "start", ID: 2}})
	if a.sessionPacks.Enabled("it") || !a.sessionPacks.Enabled("base") {
		t.Fatal("next session missed config")
	}
	var old settings
	if json.Unmarshal([]byte(`{"refinement":{"mode":"light"}}`), &old) != nil || !old.Packs.Enabled("it") || !old.Packs.Enabled("base") {
		t.Fatal("old settings did not enable default packs")
	}
}

func TestFailedPackSaveKeepsActiveConfiguration(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	a := &application{directory: file}
	a.configurePacks(wordlists.Config{ITDisabled: true})
	if a.config.Packs.ITDisabled || a.view.Snapshot.PacksFeedbackOK {
		t.Fatal("failed save changed config")
	}
}
