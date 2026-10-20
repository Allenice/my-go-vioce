package refinement

import (
	"context"
	"slices"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/mozillazg/go-pinyin"
	"github.com/vfasky/my-ai-audio/gui/internal/segmentation"
	"github.com/vfasky/my-ai-audio/gui/internal/wordlists"
)

const maxPublicOptions = 8
const maxPublicCandidates = 96

// Evidence keeps public suggestions separate from personal terms throughout
// Jev and refinement. Public words must never acquire personal protection.
type Evidence struct {
	Personal []string
	Public   []Span
	Text     string
}

// Spans recomputes personal matches, but only keeps public matches whose exact
// occurrence is still unchanged after Jev. Jev substitutions preserve rune count.
func (e Evidence) Spans(text string) []Span {
	out := Spans(text, e.Personal)
	if utf8.RuneCountInString(text) != utf8.RuneCountInString(e.Text) {
		return out
	}
	runes := []rune(text)
	for _, s := range e.Public {
		if s.Start < 0 || s.End > len(runes) || s.Start >= s.End || string(runes[s.Start:s.End]) != s.Heard {
			continue
		}
		// A public choice cannot replace an explicit personal spelling, even
		// after a previous choice introduced that spelling elsewhere.
		if protectedAt(runes, e.Personal, s.Start, s.End) {
			continue
		}
		if slices.ContainsFunc(out, func(p Span) bool { return p.Start == s.Start && p.End == s.End && p.Term == s.Term }) {
			continue
		}
		out = append(out, s)
	}
	return out
}

func protectedAt(text []rune, personal []string, start, end int) bool {
	for _, term := range personal {
		tr := []rune(term)
		if len(tr) < 2 {
			continue
		}
		for at := max(0, start-len(tr)+1); at < end && at+len(tr) <= len(text); at++ {
			if slices.Equal(text[at:at+len(tr)], tr) {
				return true
			}
		}
	}
	return false
}

// PrepareEvidence runs entirely on the worker. Personal competitors stay
// complete; public lists provide a bounded frequency-ranked shortlist.
func PrepareEvidence(ctx context.Context, text string, allTerms, hints []string, packs wordlists.Config) (Evidence, error) {
	if err := ctx.Err(); err != nil {
		return Evidence{}, err
	}
	terms, err := NewLexicon(allTerms).Retrieve(text, hints)
	if err != nil {
		return Evidence{}, err
	}
	e := Evidence{Personal: terms, Text: text}
	if packs.BaseDisabled && packs.ITDisabled {
		return e, nil
	}
	index, err := publicDictionary()
	if err != nil {
		return e, err
	}
	tokens, err := segmentation.Cut(text, terms...)
	if err != nil {
		return e, err
	}
	e.Public, err = index.candidates(ctx, text, terms, tokens, packs)
	return e, err
}

type publicIndex struct {
	entries   []wordlists.Entry
	bySound   map[soundKey][]int
	frequency map[string]map[string]int
}

var publicDictionary = sync.OnceValues(func() (*publicIndex, error) {
	entries, err := wordlists.Load()
	if err != nil {
		return nil, err
	}
	return newPublicIndex(entries), nil
})

func newPublicIndex(entries []wordlists.Entry) *publicIndex {
	index := &publicIndex{entries: entries, bySound: map[soundKey][]int{}, frequency: map[string]map[string]int{}}
	for i, entry := range entries {
		for _, first := range entry.Readings[0] {
			key := soundKey{first, len(entry.Readings)}
			index.bySound[key] = append(index.bySound[key], i)
		}
		if index.frequency[entry.Source] == nil {
			index.frequency[entry.Source] = map[string]int{}
		}
		index.frequency[entry.Source][entry.Term] = entry.Frequency
	}
	return index
}

func (index *publicIndex) candidates(ctx context.Context, text string, personal []string, tokens []segmentation.Token, packs wordlists.Config) ([]Span, error) {
	runes := []rune(text)
	if len(runes) > maxRetrievalRunes {
		return nil, ErrVocabularyLimit
	}
	boundaries := make([]bool, len(runes)+1)
	for _, token := range tokens {
		boundaries[token.Start], boundaries[token.End] = true, true
	}
	readings := make([][]string, len(runes))
	cache := map[rune][]string{}
	for i, r := range runes {
		if !unicode.Is(unicode.Han, r) {
			continue
		}
		p, ok := cache[r]
		if !ok {
			p = pinyin.SinglePinyin(r, pinyinArgs)
			cache[r] = p
		}
		readings[i] = p
	}
	var out []Span
	for start := range runes {
		if start%64 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		if !boundaries[start] || len(readings[start]) == 0 {
			continue
		}
		for length := 8; length >= 2; length-- {
			end := start + length
			if end > len(runes) || !boundaries[end] || !allHan(runes[start:end]) || protectedAt(runes, personal, start, end) {
				continue
			}
			heard := string(runes[start:end])
			var options []Span
			seen := map[string]int{}
			for _, first := range readings[start] {
				for _, id := range index.bySound[soundKey{first, length}] {
					entry := index.entries[id]
					if !packs.Enabled(entry.Source) || entry.Term == heard {
						continue
					}
					// Compare frequencies only within this source. An ordinary
					// known word is not protected, but lower-frequency alternatives
					// from the same corpus do not create new correction work.
					if index.frequency[entry.Source][heard] >= entry.Frequency {
						continue
					}
					var spelled []string
					for i, p := range entry.Readings {
						for _, reading := range p {
							if slices.Contains(readings[start+i], reading) {
								spelled = append(spelled, reading)
								break
							}
						}
						if len(spelled) != i+1 {
							break
						}
					}
					if len(spelled) != length {
						continue
					}
					s := Span{Candidate: Candidate{Heard: heard, Term: entry.Term, Pinyin: strings.Join(spelled, " "), Match: "same", Source: entry.Source, Rank: entry.Rank, Boundary: segmentation.Boundary(tokens, start, end)}, Start: start, End: end}
					if at, ok := seen[entry.Term]; ok {
						if publicOrder(s, options[at]) < 0 {
							options[at] = s
						}
					} else {
						seen[entry.Term] = len(options)
						options = append(options, s)
					}
				}
			}
			slices.SortStableFunc(options, publicOrder)
			out = append(out, options[:min(len(options), maxPublicOptions)]...)
			if len(out) > maxRetrievedSpans {
				slices.SortStableFunc(out, publicSpanOrder)
				out = out[:maxPublicCandidates]
			}
		}
	}
	// Public suggestions have their own budget and never remove a personal
	// candidate. Longer spans and IT-specific evidence come first.
	slices.SortStableFunc(out, publicSpanOrder)
	return out[:min(len(out), maxPublicCandidates)], nil
}

func publicSpanOrder(a, b Span) int {
	if d := (b.End - b.Start) - (a.End - a.Start); d != 0 {
		return d
	}
	return publicOrder(a, b)
}
func publicOrder(a, b Span) int {
	if a.Source != b.Source {
		if a.Source == "it" {
			return -1
		}
		return 1
	}
	if a.Rank != b.Rank {
		return b.Rank - a.Rank
	}
	if a.Start != b.Start {
		return a.Start - b.Start
	}
	return strings.Compare(a.Term, b.Term)
}
