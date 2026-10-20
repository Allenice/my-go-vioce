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
	spans := Spans(text, terms)
	var tokens []segmentation.Token
	if len(terms) > 0 && len([]rune(text)) <= 8000 {
		tokens, _ = segmentation.Cut(text, terms...)
	}
	if len(tokens) > 0 {
		for i := range spans {
			spans[i].Boundary = segmentation.Boundary(tokens, spans[i].Start, spans[i].End)
		}
		rank := map[string]int{"word": 0, "words": 1, "partial": 2}
		slices.SortStableFunc(spans, func(a, b Span) int { return rank[a.Boundary] - rank[b.Boundary] })
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
	runes := []rune(text)
	known := map[string]bool{}
	protected := make([]bool, len(runes))
	var han []string
	for _, t := range terms {
		t = strings.TrimSpace(t)
		known[t] = true
		tr := []rune(t)
		if len(tr) >= 2 {
			for start := 0; start+len(tr) <= len(runes); start++ {
				if slices.Equal(runes[start:start+len(tr)], tr) {
					for j := start; j < start+len(tr); j++ {
						protected[j] = true
					}
				}
			}
		}
		if r := []rune(t); len(r) >= 2 && allHan(r) && !slices.Contains(han, t) {
			han = append(han, t)
		}
	}
	slices.SortStableFunc(han, func(a, b string) int { return len([]rune(b)) - len([]rune(a)) })
	readings := make([][]string, len(runes))
	for i, r := range runes {
		if unicode.Is(unicode.Han, r) {
			readings[i] = pinyin.SinglePinyin(r, pinyinArgs)
		}
	}
	var out []Span
	for _, term := range han {
		tr := []rune(term)
		termReadings := make([][]string, len(tr))
		spelled := make([]string, len(tr))
		for i, r := range tr {
			termReadings[i] = pinyin.SinglePinyin(r, pinyinArgs)
			if len(termReadings[i]) > 0 {
				spelled[i] = termReadings[i][0]
			}
		}
		for i := 0; i+len(tr) <= len(runes); i++ {
			span := string(runes[i : i+len(tr)])
			if known[span] || slices.Contains(protected[i:i+len(tr)], true) {
				continue
			}
			match := soundMatch(readings[i:i+len(tr)], termReadings)
			if match == "" {
				continue
			}
			out = append(out, Span{Candidate{Heard: span, Term: term, Pinyin: strings.Join(spelled, " "), Match: match}, i, i + len(tr)})
		}
	}
	return out
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
