package refinement

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

// These are authored regression seeds, not recorded speech or an accuracy
// estimate. dev/eval are separate so live threshold tuning need not reuse eval.
func TestVocabularyCorpusRecall(t *testing.T) {
	data, err := os.ReadFile("testdata/vocabulary-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		ID, Split, Text, Gold, Tag string
		Terms, Recall              []string
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var fillers []string
	for i := 0; i < 2000; i++ {
		fillers = append(fillers, "测试条目"+string(rune(0x4e00+i)))
	}
	wanted, found, baselineFound := 0, 0, 0
	for _, c := range cases {
		if seen[c.ID] || c.ID == "" || c.Gold == "" || c.Tag == "" || (c.Split != "dev" && c.Split != "eval") {
			t.Fatalf("invalid case: %+v", c)
		}
		seen[c.ID] = true
		t.Run(c.ID, func(t *testing.T) {
			all := append(slices.Clone(fillers[:2000-len(c.Terms)]), c.Terms...)
			got, err := NewLexicon(all).Retrieve(c.Text, all[:100])
			if err != nil {
				t.Fatal(err)
			}
			baseline, err := NewLexicon(all[:100]).Retrieve(c.Text, all[:100])
			if err != nil {
				t.Fatal(err)
			}
			for _, term := range c.Recall {
				wanted++
				if slices.Contains(got, term) {
					found++
				} else {
					t.Errorf("missing %s in %v", term, got)
				}
				if slices.Contains(baseline, term) {
					baselineFound++
				}
			}
			// Retrieval must preserve the exact candidate set of a full search,
			// including partial matches and protection by longer known terms.
			fullSpans := Spans(c.Text, all)
			selectedSpans := Spans(c.Text, got)
			if !slices.Equal(fullSpans, selectedSpans) {
				t.Fatalf("candidate set changed: full=%+v selected=%+v", fullSpans, selectedSpans)
			}
		})
	}
	t.Logf("authored cases=%d; required term recall: full=%d/%d, Top100-only=%d/%d; no model quality measured", len(cases), found, wanted, baselineFound, wanted)
}
