package segmentation

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/yanyiwu/gojieba"
)

func TestCutPreservesUnicodeOffsetsAndConcurrentCalls(t *testing.T) {
	const text = "🙂下午我们讨论一致科技的发布计划，大家意见很一致\nGo Voice 26"
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Go(func() {
			tokens, err := Cut(text)
			if err != nil {
				t.Error(err)
				return
			}
			var b strings.Builder
			runes := []rune(text)
			end := 0
			for _, token := range tokens {
				if token.Start != end || token.End > len(runes) || string(runes[token.Start:token.End]) != token.Text {
					t.Errorf("bad offset: %+v", token)
					return
				}
				b.WriteString(token.Text)
				end = token.End
			}
			if b.String() != text {
				t.Errorf("lost text: %q", b.String())
			}
		})
	}
	wg.Wait()
}

func TestCustomWordsAreScopedAndResetBuiltInDictionary(t *testing.T) {
	text := "请打开幻羽词境后台，大家意见一致"
	base, err := Cut(text)
	if err != nil {
		t.Fatal(err)
	}
	custom, err := Cut(text, "幻羽词境", "一致", "幻羽词境", "Go Voice")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, token := range custom {
		found = found || token.Text == "幻羽词境"
	}
	if !found {
		t.Fatalf("custom term split: %+v", custom)
	}
	after, err := Cut(text)
	if err != nil || !slices.Equal(base, after) {
		t.Fatalf("dictionary leaked: before=%+v after=%+v err=%v", base, after, err)
	}
}

func TestCustomSnapshotsAreSafeWhenConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for _, word := range []string{"幻羽词境", "星潮语盒", "幻羽词境"} {
		wg.Go(func() {
			tokens, err := Cut("请打开"+word+"后台", word)
			if err != nil {
				t.Error(err)
				return
			}
			if !slices.ContainsFunc(tokens, func(token Token) bool { return token.Text == word }) {
				t.Errorf("snapshot lost %s: %+v", word, tokens)
			}
		})
	}
	wg.Wait()
}

func TestMissingDictionaryIsAnError(t *testing.T) {
	if _, err := pathsIn(t.TempDir()); err == nil {
		t.Fatal("missing dictionaries accepted")
	}
}

// Run by the package gate with the installed resource directory. This checks
// real dictionaries independently of the source module-cache location.
func TestPackagedDictionaries(t *testing.T) {
	dir := os.Getenv("GOVOICE_JIEBA_TEST_DIR")
	if dir == "" {
		t.Skip("package gate only")
	}
	paths, err := pathsIn(filepath.Clean(dir))
	if err != nil {
		t.Fatal(err)
	}
	j := gojieba.NewJieba(paths...)
	defer j.Free()
	if words := j.Cut("大家意见一致", true); strings.Join(words, "/") != "大家/意见/一致" {
		t.Fatalf("bad packaged dictionaries: %v", words)
	}
}
