// Package segmentation supplies local word boundaries for vocabulary judgments.
package segmentation

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/yanyiwu/gojieba"
)

// Token uses rune offsets, as do refinement.Span and Jev's state.
type Token struct {
	Text  string `json:"text"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

var engine struct {
	sync.Mutex
	jieba *gojieba.Jieba
	words []string
}

// Cut initializes the dictionary lazily, off the UI thread. Custom Chinese
// words belong to this snapshot only. Rebuild when the snapshot changes so
// deleted words disappear and overriding a built-in word never damages the
// base dictionary. C++ access and replacement of the cached engine are serialized.
func Cut(text string, custom ...string) ([]Token, error) {
	return cut(text, true, custom)
}

// CutKnown uses dictionary boundaries without HMM new-word guesses. Recognition
// gaps can otherwise make HMM merge a trailing fragment with the next word,
// hiding an explicit replacement. Built-in and custom words stay intact.
func CutKnown(text string, custom ...string) ([]Token, error) {
	return cut(text, false, custom)
}

func cut(text string, hmm bool, custom []string) ([]Token, error) {
	words := dictionaryWords(custom)
	engine.Lock()
	defer engine.Unlock()
	if engine.jieba == nil || !slices.Equal(engine.words, words) {
		paths, err := dictionaryPaths()
		if err != nil {
			return nil, err
		}
		jieba := gojieba.NewJieba(paths...)
		for _, word := range words {
			jieba.AddWordEx(word, 100000, "nz")
		}
		if engine.jieba != nil {
			engine.jieba.Free()
		}
		engine.jieba, engine.words = jieba, words
	}
	parts := engine.jieba.Cut(text, hmm)
	if strings.Join(parts, "") != text {
		return nil, errors.New("segmentation: incomplete text")
	}
	out := make([]Token, 0, len(parts))
	at := 0
	for _, word := range parts {
		end := at + utf8.RuneCountInString(word)
		out = append(out, Token{word, at, end})
		at = end
	}
	return out, nil
}

// ChineseWord reports whether text is a contiguous Chinese word. Phrases,
// whitespace and Latin spelling keep their existing literal matching behavior.
func ChineseWord(text string) bool {
	if utf8.RuneCountInString(text) < 2 || len(text) > 256 {
		return false
	}
	for _, r := range text {
		if !unicode.Is(unicode.Han, r) {
			return false
		}
	}
	return true
}

func dictionaryWords(custom []string) []string {
	words := make([]string, 0, len(custom))
	for _, word := range custom {
		word = strings.TrimSpace(word)
		if ChineseWord(word) {
			words = append(words, word)
		}
	}
	slices.Sort(words)
	return slices.Compact(words)
}

// Boundary describes whether a rune span covers one word, adjacent complete
// words, or part of a word. Unknown names can still cross token boundaries.
func Boundary(tokens []Token, start, end int) string {
	left, right := false, false
	for _, token := range tokens {
		if start == token.Start && end == token.End {
			return "word"
		}
		left = left || start == token.Start
		right = right || end == token.End
	}
	if left && right {
		return "words"
	}
	return "partial"
}

func dictionaryPaths() ([]string, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(filepath.Dir(exe), "..", "Resources", "jieba")
	// A packaged app must use its own resources, never the build machine's
	// module cache. Source builds and unit tests use the pinned Go module.
	if filepath.Base(filepath.Dir(exe)) != "MacOS" {
		dir = gojieba.DICT_DIR
	}
	return pathsIn(dir)
}

func pathsIn(dir string) ([]string, error) {
	var paths []string
	for _, name := range []string{"jieba.dict.utf8", "hmm_model.utf8", "user.dict.utf8", "idf.utf8", "stop_words.utf8"} {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return nil, errors.New("segmentation: dictionary unavailable")
		}
		paths = append(paths, path)
	}
	return paths, nil
}
