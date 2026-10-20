package refinement

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/mozillazg/go-pinyin"
)

func TestRetrieveFullVocabularyAndKeepCompetitors(t *testing.T) {
	terms := make([]string, 0, 2000)
	for i := 0; i < 1995; i++ {
		terms = append(terms, "测试条目"+string(rune(0x4e00+i)))
	}
	terms = append(terms, "伊智", "益智", "私域宝", "不相关词", "Go Voice")
	l := NewLexicon(terms)
	got, err := l.Retrieve("🙂打开施玉宝，再联系一致科技", terms[:100])
	if err != nil || !slices.Equal(got, []string{"伊智", "益智", "私域宝"}) {
		t.Fatalf("retrieval: %v %v", got, err)
	}
	spans, _, err := l.match("🙂打开施玉宝，再联系一致科技")
	if err != nil || !reflect.DeepEqual(spans, Spans("🙂打开施玉宝，再联系一致科技", got)) {
		t.Fatalf("competitors changed after retrieval: %+v %v", spans, err)
	}
}

func TestRetrieveProtectsKnownTermsAndRetainsLatinHints(t *testing.T) {
	terms := []string{"伊智", "一致性", "Go Voice", "OpenAI", "额外词条"}
	l := NewLexicon(terms)
	got, err := l.Retrieve("保持一致性，用 OPENAI", []string{"Go Voice", "额外词条"})
	if err != nil || !slices.Equal(got, []string{"一致性", "Go Voice", "OpenAI"}) {
		t.Fatalf("retrieval: %v %v", got, err)
	}
	if spans := Spans("保持一致性，用 OPENAI", got); len(spans) != 0 {
		t.Fatal("protected text gained a candidate", spans)
	}
	terms[1] = "新词"
	got, _ = l.Retrieve("保持一致性", nil)
	if !slices.Equal(got, []string{"一致性"}) {
		t.Fatal("index changed with source slice", got)
	}
}

func TestRetrieveDeclinesOverflowWithoutChoosingSomeHomophones(t *testing.T) {
	var terms []string
	chars := []rune("衣医依伊")
	for i := 0; i <= maxRetrievedTerms; i++ {
		n := i
		var term []rune
		for range 5 {
			term = append(term, chars[n%4])
			n /= 4
		}
		terms = append(terms, string(term))
	}
	l := NewLexicon(terms)
	if got, err := l.Retrieve("一一一一一", nil); !errors.Is(err, ErrVocabularyLimit) || len(got) != 0 {
		t.Fatalf("truncated competitors: %v %v", got, err)
	}
	if got, err := l.Retrieve(strings.Repeat("前", maxRetrievalRunes+1), nil); !errors.Is(err, ErrVocabularyLimit) || len(got) != 0 {
		t.Fatalf("truncated text: %v %v", got, err)
	}
	l = NewLexicon([]string{"伊智", "益智", "意志"})
	if got, err := l.Retrieve(strings.Repeat("一致，", 1400), nil); !errors.Is(err, ErrVocabularyLimit) || len(got) != 0 {
		t.Fatalf("truncated occurrences: %v %v", got, err)
	}
}

// A deliberately simple exhaustive oracle checks the index independently. It
// preserves all readings and protected words, including overlapping literals.
func exhaustiveSpans(text string, terms []string) []Span {
	runes := []rune(text)
	l := NewLexicon(terms)
	protected := make([]bool, len(runes))
	for _, term := range l.terms {
		if len(term.runes) < 2 {
			continue
		}
		for at := 0; at+len(term.runes) <= len(runes); at++ {
			if slices.Equal(runes[at:at+len(term.runes)], term.runes) {
				for i := at; i < at+len(term.runes); i++ {
					protected[i] = true
				}
			}
		}
	}
	var out []Span
	for _, term := range l.terms {
		if len(term.readings) == 0 {
			continue
		}
		for at := 0; at+len(term.runes) <= len(runes); at++ {
			end := at + len(term.runes)
			if slices.Contains(protected[at:end], true) {
				continue
			}
			var readings [][]string
			for _, r := range runes[at:end] {
				var p []string
				if unicode.Is(unicode.Han, r) {
					p = pinyin.SinglePinyin(r, pinyinArgs)
				}
				readings = append(readings, p)
			}
			if match := soundMatch(readings, term.readings); match != "" {
				out = append(out, Span{Candidate: Candidate{Heard: string(runes[at:end]), Term: term.text, Pinyin: term.pinyin, Match: match}, Start: at, End: end})
			}
		}
	}
	slices.SortStableFunc(out, func(a, b Span) int { return (b.End - b.Start) - (a.End - a.Start) })
	return out
}

func TestIndexMatchesExhaustivePolyphonicAndBoundarySearch(t *testing.T) {
	terms := []string{"伊智", "益智", "私域宝", "石玉", "上海虹桥", "心仪", "重庆", "银行", "南京", "一致性", "一致性检查", "Go Voice", "智", "伊智"}
	for _, text := range []string{
		"🙂打开施玉宝，一致科技与上海洪桥", "一心一意，一字，一直，一致性检查",
		"重情，银杭，蓝鲸，重庆，银行", "Go Voice和伊智，保持一致性", "施 玉宝。伊，智", "", "🙂a1\n",
	} {
		if got, want := Spans(text, terms), exhaustiveSpans(text, terms); !reflect.DeepEqual(got, want) {
			t.Fatalf("%q\ngot  %+v\nwant %+v", text, got, want)
		}
	}
}

func TestRefinementDeclinesOversizedPromptBeforeCallingModel(t *testing.T) {
	text := strings.Repeat("a", 128*1024)
	called := false
	out, status := Run(context.Background(), Config{Engine: "local", Mode: "names"}, text, nil, nil, func(context.Context, string) (string, error) {
		called = true
		return "changed", nil
	})
	if called || out != text || !strings.Contains(status, "过长") {
		t.Fatalf("oversized prompt was sent: called=%v status=%s", called, status)
	}
}

func BenchmarkVocabularyRetrieval(b *testing.B) {
	for _, count := range []int{100, 2000, 4000} {
		var terms []string
		for i := 0; i < count-2; i++ {
			terms = append(terms, "测试条目"+string(rune(0x4e00+i)))
		}
		terms = append(terms, "伊智", "私域宝")
		text := strings.Repeat("今天我们讨论产品发布计划，", 6) + "打开施玉宝后台联系一致科技"
		b.Run(fmt.Sprintf("cold/%d", count), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := NewLexicon(terms).Retrieve(text, nil); err != nil {
					b.Fatal(err)
				}
			}
		})
		l := NewLexicon(terms)
		b.Run(fmt.Sprintf("lookup/%d", count), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := l.Retrieve(text, nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
