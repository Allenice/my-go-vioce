package refinement

import (
	"slices"
	"strings"
	"unicode"

	"github.com/mozillazg/go-pinyin"
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
}

// maxCandidates bounds the hints of one prompt.
const maxCandidates = 24

var pinyinArgs = func() pinyin.Args {
	a := pinyin.NewArgs()
	a.Heteronym = true // every reading of a character, without tones
	return a
}()

// Candidates finds the spans of text that sound like a term of the
// vocabulary, longest terms first. Only terms of two or more Chinese
// characters are compared; spans that already are a vocabulary term are
// skipped.
func Candidates(text string, terms []string) []Candidate {
	runes := []rune(text)
	known := map[string]bool{}
	var han []string
	for _, t := range terms {
		t = strings.TrimSpace(t)
		known[t] = true
		if r := []rune(t); len(r) >= 2 && allHan(r) {
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
	var out []Candidate
	seen := map[string]bool{}
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
			if known[span] || seen[span+"\x00"+term] {
				continue
			}
			match := soundMatch(readings[i:i+len(tr)], termReadings)
			if match == "" {
				continue
			}
			seen[span+"\x00"+term] = true
			out = append(out, Candidate{Heard: span, Term: term, Pinyin: strings.Join(spelled, " "), Match: match})
			if len(out) == maxCandidates {
				return out
			}
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
