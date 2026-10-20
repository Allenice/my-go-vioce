package refinement

import (
	"strings"
	"testing"
)

func TestVocabularySegmentationHelpsRefinementWithoutJev(t *testing.T) {
	prompt := Prompt("打开幻羽词境，再看一致科技", []string{"幻羽词境", "伊智"}, "names")
	for _, part := range []string{SegmentationRule, `"tokens":`, `"text":"幻羽词境"`, `"heard":"一致"`, `"boundary":"word"`} {
		if !strings.Contains(prompt, part) {
			t.Fatalf("missing %q: %s", part, prompt)
		}
	}
}

func TestCandidatesProtectLongKnownTermsAndRetainPartialHints(t *testing.T) {
	if candidates := Candidates("保持一致性", []string{"一致性", "伊智"}); len(candidates) != 0 {
		t.Fatalf("known term cut: %+v", candidates)
	}
	candidates := Candidates("一心一意，与一致科技沟通", []string{"心仪", "伊智"})
	if len(candidates) != 2 || candidates[0].Heard != "一致" || candidates[0].Boundary != "word" || candidates[1].Boundary != "partial" {
		t.Fatalf("ranking: %+v", candidates)
	}
}
