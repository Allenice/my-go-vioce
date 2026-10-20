package refinement

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mozillazg/go-pinyin"
	"github.com/vfasky/my-ai-audio/gui/internal/segmentation"
	"github.com/vfasky/my-ai-audio/gui/internal/wordlists"
)

func TestPublicDictionaryDoesNotProtectOrdinaryWords(t *testing.T) {
	text := "联系一致科技，大家意见一致"
	e, err := PrepareEvidence(context.Background(), text, []string{"伊智"}, nil, wordlists.Config{})
	if err != nil {
		t.Fatal(err)
	}
	spans := e.Spans(text)
	personal := 0
	for _, s := range spans {
		if s.Term == "伊智" && s.Source == "" {
			personal++
		}
	}
	if personal != 2 {
		t.Fatalf("public 一致 suppressed personal name: %+v", spans)
	}
	if slices.Contains(e.Personal, "一致") {
		t.Fatal("public word became a personal term")
	}
}

func TestPublicITRecallWithoutPersonalVocabulary(t *testing.T) {
	for _, text := range []string{"先进行初始话再处理数据", "这是一个字符船", "这个对象需要实例话"} {
		e, err := PrepareEvidence(context.Background(), text, nil, nil, wordlists.Config{BaseDisabled: true})
		if err != nil {
			t.Fatal(err)
		}
		want := "初始化"
		if strings.Contains(text, "字符") {
			want = "字符串"
		}
		if strings.Contains(text, "实例") {
			want = "实例化"
		}
		if !slices.ContainsFunc(e.Public, func(s Span) bool { return s.Term == want && s.Source == "it" }) {
			t.Fatalf("%s missing %s: %+v", text, want, e.Public)
		}
		if len(e.Personal) != 0 {
			t.Fatal("public terms entered personal vocabulary")
		}
		prompt := PromptEvidence(text, e, "names")
		if !strings.Contains(prompt, PublicVocabularyRule) || !strings.Contains(prompt, `"source":"it"`) || !strings.Contains(prompt, `"term":"`+want+`"`) {
			t.Fatalf("missing source evidence: %s", prompt)
		}
	}
}

func TestPublicPacksDisableAndProtectPersonalNames(t *testing.T) {
	text := "先进行初始话"
	e, err := PrepareEvidence(context.Background(), text, nil, nil, wordlists.Config{BaseDisabled: true, ITDisabled: true})
	if err != nil || len(e.Public) != 0 {
		t.Fatalf("disabled: %+v %v", e, err)
	}
	e, err = PrepareEvidence(context.Background(), text, []string{"初始话"}, nil, wordlists.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, span := range e.Spans(text) {
		if span.Start < 6 && span.End > 3 {
			t.Fatalf("personal spelling changed: %+v", span)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := PrepareEvidence(ctx, text, nil, nil, wordlists.Config{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestPublicEvidenceRetainsOccurrenceAndDoesNotRevertJev(t *testing.T) {
	text := "初始话完成，再次初始话"
	e, err := PrepareEvidence(context.Background(), text, nil, nil, wordlists.Config{BaseDisabled: true})
	if err != nil {
		t.Fatal(err)
	}
	after := "初始化完成，再次初始话"
	for _, s := range e.Spans(after) {
		if s.Start == 0 {
			t.Fatalf("changed occurrence still offered: %+v", s)
		}
	}
	prompt := PromptEvidence(after, e, "names", "初始话")
	var fields struct {
		Candidates []Candidate
		Keep       []string
		Vocabulary []string
	}
	at := strings.LastIndex(prompt, "\n")
	if err := json.Unmarshal([]byte(prompt[at+1:]), &fields); err != nil {
		t.Fatal(err)
	}
	for _, c := range fields.Candidates {
		if c.Heard == "初始话" {
			t.Fatal("confident keep discarded")
		}
	}
	if !slices.Equal(fields.Keep, []string{"初始话"}) || len(fields.Vocabulary) != 0 {
		t.Fatalf("evidence leaked: %+v", fields)
	}
}

func TestPublicSuggestionsUseBoundariesAndSameSourceFrequency(t *testing.T) {
	makeEntry := func(term, source string, freq, rank int) wordlists.Entry {
		e := wordlists.Entry{Term: term, Source: source, Frequency: freq, Rank: rank}
		for _, r := range term {
			e.Readings = append(e.Readings, pinyin.SinglePinyin(r, pinyinArgs))
		}
		return e
	}
	index := newPublicIndex([]wordlists.Entry{makeEntry("一致", "base", 100, 90), makeEntry("益智", "base", 10, 60), makeEntry("伊智", "it", 5, 30), makeEntry("心仪", "base", 50, 70)})
	text := "一致一心一意"
	tokens := []segmentation.Token{{Text: "一致", Start: 0, End: 2}, {Text: "一心一意", Start: 2, End: 6}}
	spans, err := index.candidates(context.Background(), text, nil, tokens, wordlists.Config{})
	if err != nil || len(spans) != 1 || spans[0].Term != "伊智" {
		t.Fatalf("cross-source frequency/protection or token cut: %+v %v", spans, err)
	}
}

func BenchmarkPublicVocabulary(b *testing.B) {
	text := "请先进行初始话，再把字符船转换成数组，联系一致科技确认接口参数。"
	index, err := publicDictionary()
	if err != nil {
		b.Fatal(err)
	}
	tokens, err := segmentation.Cut(text, "伊智")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := index.candidates(context.Background(), text, []string{"伊智"}, tokens, wordlists.Config{}); err != nil {
			b.Fatal(err)
		}
	}
}
