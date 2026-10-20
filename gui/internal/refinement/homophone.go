package refinement

import (
	"slices"
	"strings"
	"unicode"

	"github.com/mozillazg/go-pinyin"
	"github.com/vfasky/my-ai-audio/gui/internal/segmentation"
)

// Candidate is a span of the dictation that sounds like a Chinese
// vocabulary term: speech recognition writes a name it does not know as
// common characters of the same sound, such as 一致 or 一直 for 伊智.
type Candidate struct {
	Heard  string `json:"heard"`
	Term   string `json:"term"`
	Pinyin string `json:"pinyin"`
	// Match is "same" for the same pinyin, "similar" for a sound that only
	// differs in z/zh, c/ch, s/sh, n/l or a final -n/-ng.
	Match string `json:"match"`
	// Boundary is a segmentation hint, never permission to replace a word.
	Boundary string `json:"boundary,omitempty"`
	// Public sources are background suggestions, never personal-name protection.
	Source string `json:"source,omitempty"`
	Rank   int    `json:"rank,omitempty"`
}

// maxCandidates bounds the hints of one prompt.
const maxCandidates = 24

var pinyinArgs = func() pinyin.Args {
	a := pinyin.NewArgs()
	a.Heteronym = true // every reading of a character, without tones
	return a
}()

// Span is a Candidate at its place in the dictation: runes Start to End.
type Span struct {
	Candidate
	Start, End int
}

// Candidates finds spans that sound like a vocabulary term, ranking complete
// word boundaries first and longer terms first within each boundary class.
// Only terms of two or more Chinese characters are compared; correctly spelled
// vocabulary terms, including shorter spans inside them, are protected.
func Candidates(text string, terms []string) []Candidate {
	candidates, _ := candidateAnalysis(text, terms)
	return candidates
}

func candidateAnalysis(text string, terms []string) ([]Candidate, []segmentation.Token) {
	return candidateEvidenceAnalysis(text, Evidence{Personal: terms, Text: text})
}

func candidateEvidenceAnalysis(text string, evidence Evidence) ([]Candidate, []segmentation.Token) {
	spans := evidence.Spans(text)
	var tokens []segmentation.Token
	if (len(evidence.Personal) > 0 || len(evidence.Public) > 0) && len([]rune(text)) <= 8000 {
		tokens, _ = segmentation.Cut(text, evidence.Personal...)
	}
	if len(tokens) > 0 {
		for i := range spans {
			spans[i].Boundary = segmentation.Boundary(tokens, spans[i].Start, spans[i].End)
		}
		rank := map[string]int{"word": 0, "words": 1, "partial": 2}
		slices.SortStableFunc(spans, func(a, b Span) int {
			if (a.Source == "") != (b.Source == "") {
				if a.Source == "" {
					return -1
				}
				return 1
			}
			return rank[a.Boundary] - rank[b.Boundary]
		})
	}
	var out []Candidate
	seen := map[string]bool{}
	for _, s := range spans {
		if seen[s.Heard+"\x00"+s.Term] {
			continue
		}
		seen[s.Heard+"\x00"+s.Term] = true
		out = append(out, s.Candidate)
		if len(out) == maxCandidates {
			break
		}
	}
	return out, tokens
}

// Spans is Candidates at every place they occur, so that each can be
// judged and replaced on its own.
func Spans(text string, terms []string) []Span {
	spans, _, _ := NewLexicon(terms).match(text)
	return spans
}

// soundMatch compares the readings of a span with those of a term
// character by character: "same" when every pair shares a reading,
// "similar" when every pair does after folding similar sounds.
func soundMatch(span, term [][]string) string {
	same := true
	for i := range term {
		if len(span[i]) == 0 || len(term[i]) == 0 {
			return ""
		}
		if !shares(span[i], term[i], func(s string) string { return s }) {
			same = false
			if !shares(span[i], term[i], foldSound) {
				return ""
			}
		}
	}
	if same {
		return "same"
	}
	return "similar"
}

func shares(a, b []string, fold func(string) string) bool {
	for _, x := range a {
		for _, y := range b {
			if fold(x) == fold(y) {
				return true
			}
		}
	}
	return false
}

// foldSound merges the sounds that speech recognition and many speakers
// confuse: z/zh, c/ch, s/sh, n/l, and the finals -an/-ang, -en/-eng and
// -in/-ing.
func foldSound(s string) string {
	for _, p := range [][2]string{{"zh", "z"}, {"ch", "c"}, {"sh", "s"}, {"l", "n"}} {
		if rest, ok := strings.CutPrefix(s, p[0]); ok {
			s = p[1] + rest
			break
		}
	}
	for _, p := range [][2]string{{"ang", "an"}, {"eng", "en"}, {"ing", "in"}} {
		if rest, ok := strings.CutSuffix(s, p[0]); ok {
			s = rest + p[1]
			break
		}
	}
	return s
}

func allHan(r []rune) bool {
	for _, c := range r {
		if !unicode.Is(unicode.Han, c) {
			return false
		}
	}
	return true
}
