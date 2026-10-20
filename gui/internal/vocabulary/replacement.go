package vocabulary

import (
	"errors"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Replacement corrects a misrecognition: From is what speech recognition
// writes, such as 施玉宝, To what the user said, such as 私域宝.
type Replacement struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	CreatedAt time.Time `json:"createdAt"`
}

// Limits of the replacements.
const (
	maxReplacements = 2000
	maxFromLength   = 64  // runes
	maxToLength     = 256 // bytes
)

// Why a replacement was refused.
var (
	ErrInvalidReplacement = errors.New("invalid replacement")
	ErrReplacementIsTerm  = errors.New("replacement source is a term")
	ErrReplacementTaken   = errors.New("replacement source exists")
)

// PutReplacement saves r in place of the replacement of previous, which an
// edit may change, or as a new one when previous is empty.
func (s *Store) PutReplacement(previous string, r Replacement) error {
	r.From = strings.Join(strings.Fields(r.From), " ")
	r.To = strings.TrimSpace(r.To)
	from := Normalize(r.From)
	switch {
	case from == "" || r.To == "" || from == Normalize(r.To),
		!utf8.ValidString(r.From) || !utf8.ValidString(r.To),
		utf8.RuneCountInString(r.From) > maxFromLength || len(r.To) > maxToLength:
		return ErrInvalidReplacement
	}
	for _, t := range s.Terms {
		if t.NormalizedTerm == from {
			return ErrReplacementIsTerm
		}
	}
	at := -1
	for i, v := range s.Replacements {
		switch n := Normalize(v.From); {
		case previous != "" && n == Normalize(previous):
			at = i
		case n == from:
			return ErrReplacementTaken
		}
	}
	if at >= 0 {
		r.CreatedAt = s.Replacements[at].CreatedAt
		s.Replacements[at] = r
		return nil
	}
	if len(s.Replacements) >= maxReplacements {
		return ErrInvalidReplacement
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	s.Replacements = append(s.Replacements, r)
	return nil
}

// DeleteReplacement removes the replacement of from.
func (s *Store) DeleteReplacement(from string) {
	s.Replacements = slices.DeleteFunc(s.Replacements, func(r Replacement) bool { return Normalize(r.From) == Normalize(from) })
}

// maxPasses bounds how often Correct applies the replacements again to
// what an earlier pass produced.
const maxPasses = 8

// Correct applies the replacements to text until nothing changes, so that
// what one rule writes next to the dictation is corrected by another
// (公寓托客 becomes 公域托客, then 公域拓客). A later pass never matches
// inside text that replacements wrote entirely, so a target that contains
// its source, as Go Voice for go, is written once, and rules that undo each
// other stop. Within a pass the longest source wins where several start at
// the same place. Letters compare without case, and a source that starts
// or ends with an ASCII letter or digit only matches a whole word, so that
// "go" leaves "google" alone.
func (s *Store) Correct(text string) string {
	return s.correct(text, false)
}

// CorrectSegmented also recognizes Chinese rule sources separated by horizontal
// whitespace, at complete word boundaries. Call on a worker: it may load Jieba.
func (s *Store) CorrectSegmented(text string) string {
	return s.correct(text, true)
}

func (s *Store) correct(text string, segmented bool) string {
	if len(s.Replacements) == 0 {
		return text
	}
	byFirst := map[rune][]Replacement{}
	for _, r := range s.Replacements {
		first, _ := utf8.DecodeRuneInString(r.From)
		first = unicode.ToLower(first)
		byFirst[first] = append(byFirst[first], r)
	}
	for _, rs := range byFirst {
		sort.SliceStable(rs, func(i, j int) bool { return len(rs[i].From) > len(rs[j].From) })
	}
	written := make([]bool, len(text)) // bytes a replacement wrote
	for range maxPasses {
		next, nextWritten := correctOnce(text, written, byFirst)
		if segmented && s.NeedsSegmentation(next) {
			next, nextWritten = s.correctSpaced(next, nextWritten, byFirst)
		}
		if next == text {
			break
		}
		text, written = next, nextWritten
	}
	return text
}

// correctOnce applies the replacements to text in one pass, skipping
// matches whose every byte a replacement wrote, and returns the text and
// which of its bytes replacements wrote.
func correctOnce(text string, written []bool, byFirst map[rune][]Replacement) (string, []bool) {
	var b strings.Builder
	out := make([]bool, 0, len(text))
	for i := 0; i < len(text); {
		c, size := utf8.DecodeRuneInString(text[i:])
		replaced := false
		for _, r := range byFirst[unicode.ToLower(c)] {
			end := i + len(r.From)
			if end > len(text) || !strings.EqualFold(text[i:end], r.From) || !wordEdge(text, i, end, r.From) || selfExpansion(text, i, r) || !slices.Contains(written[i:end], false) {
				continue
			}
			b.WriteString(r.To)
			for range len(r.To) {
				out = append(out, true)
			}
			i, replaced = end, true
			break
		}
		if !replaced {
			b.WriteString(text[i : i+size])
			out = append(out, written[i:i+size]...)
			i += size
		}
	}
	return b.String(), out
}

// selfExpansion recognizes a rule's own longer target around its source. Live
// correction and final correction run separately, so byte ownership alone cannot
// stop an already correct "Go Voice" expanding again under go -> Go Voice.
// This affects only this rule; an explicitly configured rule for the target can
// still match it, and cycles between different words retain their old behavior.
func selfExpansion(text string, start int, rule Replacement) bool {
	if len(rule.To) <= len(rule.From) {
		return false
	}
	for offset := range rule.To {
		end := offset + len(rule.From)
		if end > len(rule.To) || !strings.EqualFold(rule.To[offset:end], rule.From) {
			continue
		}
		from, to := start-offset, start-offset+len(rule.To)
		if from >= 0 && to <= len(text) && strings.EqualFold(text[from:to], rule.To) && wordEdge(text, from, to, rule.To) {
			return true
		}
	}
	return false
}

// wordEdge reports whether text[start:end], a match of from, does not cut
// an ASCII word at an edge where from has an ASCII letter or digit.
func wordEdge(text string, start, end int, from string) bool {
	if first, _ := utf8.DecodeRuneInString(from); asciiWord(first) && start > 0 {
		if before, _ := utf8.DecodeLastRuneInString(text[:start]); asciiWord(before) {
			return false
		}
	}
	if last, _ := utf8.DecodeLastRuneInString(from); asciiWord(last) && end < len(text) {
		if after, _ := utf8.DecodeRuneInString(text[end:]); asciiWord(after) {
			return false
		}
	}
	return true
}

func asciiWord(r rune) bool {
	return r < utf8.RuneSelf && (unicode.IsLetter(r) || unicode.IsDigit(r))
}
