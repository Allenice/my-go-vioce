package wordlists

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"slices"
	"testing"
)

func TestBundledPacks(t *testing.T) {
	entries, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, entry := range entries {
		counts[entry.Source]++
		if entry.Rank < 1 || entry.Rank > 100 || len(entry.Readings) < 2 {
			t.Fatalf("invalid entry: %+v", entry)
		}
	}
	for _, source := range Catalog() {
		if counts[source.ID] != source.Terms || source.License != "MIT" || len(source.InputSHA256) != 64 {
			t.Fatalf("manifest mismatch: %+v", source)
		}
	}
	if counts["base"] != 96336 || counts["it"] != 12854 {
		t.Fatal(counts)
	}
	if !slices.ContainsFunc(entries, func(e Entry) bool { return e.Source == "it" && e.Term == "初始化" }) {
		t.Fatal("missing IT term")
	}
	if !slices.ContainsFunc(entries, func(e Entry) bool { return e.Source == "base" && e.Term == "一致" }) {
		t.Fatal("missing ordinary word")
	}
}

func TestPackDefaultsAndDisablePersist(t *testing.T) {
	for _, raw := range []string{`{}`, `{"base_disabled":false,"it_disabled":false}`} {
		var c Config
		if json.Unmarshal([]byte(raw), &c) != nil || !c.Enabled("base") || !c.Enabled("it") || c.Enabled("finance") {
			t.Fatalf("defaults: %+v", c)
		}
	}
	want := Config{ITDisabled: true}
	b, _ := json.Marshal(want)
	var got Config
	if json.Unmarshal(b, &got) != nil || got != want || got.Enabled("it") || !got.Enabled("base") {
		t.Fatalf("round trip: %s", b)
	}
}

func TestMalformedDataRejected(t *testing.T) {
	for _, text := range []string{
		"初始化\t5\tchu shi\n",
		"初始化\tbad\tchu shi hua\n",
		"初始化\t5\tchu shi !!\n",
		"初始化\t5\tchu shi hua\n初始化\t5\tchu shi hua\n",
	} {
		var b bytes.Buffer
		w := gzip.NewWriter(&b)
		_, _ = w.Write([]byte(text))
		_ = w.Close()
		if _, err := decode(b.Bytes(), Source{ID: "it", Terms: 1, MinFrequency: 1}); err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
}
