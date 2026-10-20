package vocabulary

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/vfasky/my-ai-audio/gui/internal/segmentation"
)

// NeedsSegmentation is a cheap check suitable for the UI thread. The expensive
// pass runs only for Chinese rule sources and horizontal gaps between Han text.
func (s *Store) NeedsSegmentation(text string) bool {
	if utf8.RuneCountInString(text) > 8000 {
		return false
	}
	possible := false
	for _, rule := range s.Replacements {
		if segmentation.ChineseWord(rule.From) {
			possible = true
			break
		}
	}
	if !possible {
		return false
	}
	beforeHan, gap := false, false
	for _, r := range text {
		if horizontal(r) {
			gap = gap || beforeHan
			continue
		}
		if gap && unicode.Is(unicode.Han, r) {
			return true
		}
		beforeHan, gap = unicode.Is(unicode.Han, r), false
	}
	return false
}

func horizontal(r rune) bool { return r == '\t' || unicode.Is(unicode.Zs, r) }

func hanToken(text string) bool {
	if text == "" {
		return false
	}
	for _, r := range text {
		if !unicode.Is(unicode.Han, r) {
			return false
		}
	}
	return true
}

func gapToken(text string) bool {
	if text == "" {
		return false
	}
	for _, r := range text {
		if !horizontal(r) {
			return false
		}
	}
	return true
}

func (s *Store) correctSpaced(text string, written []bool, byFirst map[rune][]Replacement) (string, []bool) {
	tokens, err := segmentation.CutKnown(text, s.Contextual(100)...)
	if err != nil {
		return text, written
	}
	// Token positions are rune offsets; the literal engine tracks byte ownership.
	offsets := make([]int, 0, utf8.RuneCountInString(text)+1)
	for at := range text {
		offsets = append(offsets, at)
	}
	offsets = append(offsets, len(text))
	var b strings.Builder
	out := make([]bool, 0, len(text))
	for i := 0; i < len(tokens); {
		token := tokens[i]
		start, end := offsets[token.Start], offsets[token.End]
		bestEnd, target := -1, ""
		if hanToken(token.Text) {
			first, _ := utf8.DecodeRuneInString(token.Text)
			// The index is longest-first. Every consumed Han token must be
			// complete; punctuation, line breaks and Latin words stop a match.
			for _, rule := range byFirst[first] {
				if !segmentation.ChineseWord(rule.From) {
					continue
				}
				joined, gap := "", false
				for j := i; j < len(tokens); j++ {
					part := tokens[j].Text
					if gapToken(part) {
						gap = true
						continue
					}
					if !hanToken(part) {
						break
					}
					joined += part
					if !strings.HasPrefix(rule.From, joined) {
						break
					}
					if joined == rule.From {
						to := offsets[tokens[j].End]
						if gap && slices.Contains(written[start:to], false) {
							bestEnd, target = j+1, rule.To
						}
						break
					}
				}
				if bestEnd >= 0 {
					break
				}
			}
		}
		if bestEnd >= 0 {
			b.WriteString(target)
			for range len(target) {
				out = append(out, true)
			}
			i = bestEnd
		} else {
			b.WriteString(text[start:end])
			out = append(out, written[start:end]...)
			i++
		}
	}
	return b.String(), out
}
