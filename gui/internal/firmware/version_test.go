package firmware

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRelate(t *testing.T) {
	cases := []struct {
		device, bundled string
		want            Relation
	}{
		{"0.1.0", "0.2.0", Older},
		{"0.2.0", "0.2.0", Equal},
		{"0.2.1", "0.2.0", Newer},
		{"0.9.0", "0.10.0", Older},
		{"1.0.0", "0.9.9", Newer},
		{"", "0.2.0", Unknown},
		{"0.2.0", "", Unknown},
		{"v0.2.0", "0.2.0", Unknown},
		{"0.2", "0.2.0", Unknown},
		{"0.02.0", "0.2.0", Unknown},
	}
	for _, c := range cases {
		if got := Relate(c.device, c.bundled); got != c.want {
			t.Errorf("Relate(%q,%q)=%d want %d", c.device, c.bundled, got, c.want)
		}
	}
	if !NeedsUpdate("0.1.0", "0.2.0") || NeedsUpdate("0.2.0", "0.2.0") || NeedsUpdate("0.3.0", "0.2.0") {
		t.Fatal("NeedsUpdate")
	}
}

func TestReadVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	if ReadVersion(path) != "" {
		t.Fatal("missing")
	}
	if err := os.WriteFile(path, []byte(`{"firmwareVersion":"0.2.0","product":"Go Voice"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := ReadVersion(path); got != "0.2.0" {
		t.Fatalf("got %q", got)
	}
	if err := os.WriteFile(path, []byte(`{"firmwareVersion":"v0.2.0"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if ReadVersion(path) != "" {
		t.Fatal("rejected v-prefix")
	}
}
