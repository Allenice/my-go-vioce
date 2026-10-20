package jev

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/vfasky/my-ai-audio/gui/internal/refinement"
	"github.com/vfasky/my-ai-audio/gui/internal/segmentation"
)

func TestChoiceComparesHomophonesAndOverlappingNames(t *testing.T) {
	text := "🙂打开施玉宝后台，再看一致科技，大家意见一致"
	spans := refinement.Spans(text, []string{"私域宝", "石玉", "伊智", "益智"})
	groups := groupSpans(spans)
	if len(groups) != 3 || len(groups[0].Spans) != 2 || len(groups[1].Spans) != 2 || len(groups[2].Spans) != 2 {
		t.Fatalf("groups: %+v", groups)
	}
	tokens, err := segmentation.Cut(text)
	if err != nil {
		t.Fatal(err)
	}
	body, names := request("jev-latest", text, groups, tokens)
	var req struct {
		State struct {
			Dictation string
			Tokens    []segmentation.Token
			Spans     map[string]struct {
				Options map[string]struct {
					Term, Replacement, Boundary string
					Start, End                  int
				}
			}
		}
		Questions map[string]question
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}
	if req.State.Dictation != text || len(req.State.Tokens) == 0 {
		t.Fatal("missing context")
	}
	response := map[string]any{}
	for i, name := range names {
		g := req.State.Spans[name]
		ps := map[string]float64{"keep": 0.03}
		winner := ""
		for key, c := range g.Options {
			if c.Boundary == "" || c.Start < groups[i].Start || c.End > groups[i].End {
				t.Fatalf("option: %+v", c)
			}
			ps[key] = 0.05
			if (i == 0 && c.Term == "私域宝") || (i == 1 && c.Term == "伊智") {
				ps[key], winner = 0.92, key
			}
			if i == 0 && c.Term == "石玉" && c.Replacement != "石玉宝" {
				t.Fatalf("lost suffix: %+v", c)
			}
		}
		if i == 2 {
			ps["keep"], winner = 0.9, "keep"
		}
		if len(req.Questions[name].Criteria) != len(g.Options)+1 {
			t.Fatal("missing choice")
		}
		response[name] = map[string]any{"type": "choice", "choice": winner, "probabilities": ps}
	}
	b, _ := json.Marshal(map[string]any{"answers": response})
	decisions, err := answers(b, names, groups)
	if err != nil {
		t.Fatal(err)
	}
	res := apply(text, decisions)
	if res.Text != "🙂打开私域宝后台，再看伊智科技，大家意见一致" || !slices.Equal(res.Keep, []string{"一致"}) {
		t.Fatalf("result: %+v", res)
	}
}

func TestCheckUsesCompleteRetrievedCompetitorsBeyondTop100(t *testing.T) {
	cfg, seen := serve(t, func(string, string) float64 { return 0.04 })
	var all []string
	for i := 0; i < 120; i++ {
		all = append(all, "未命中条目"+string(rune(0x4e00+i)))
	}
	all = append(all, "伊智", "益智", "私域宝")
	text := "大家的意见一致"
	terms, err := refinement.NewLexicon(all).Retrieve(text, all[:100])
	if err != nil || !slices.Equal(terms, []string{"伊智", "益智"}) {
		t.Fatalf("retrieval: %v %v", terms, err)
	}
	res, err := Check(context.Background(), cfg, text, terms, "test-key")
	if err != nil || res.Text != text || len(res.Decisions) != 2 || len(*seen) != 1 {
		t.Fatalf("competing choices: %+v %v", res, err)
	}
	body, _ := json.Marshal((*seen)[0])
	if strings.Contains(string(body), "未命中条目") || strings.Contains(string(body), "私域宝") {
		t.Fatal("unrelated vocabulary sent")
	}
}

func TestDiffuseChoicesDoNotMeanKeep(t *testing.T) {
	groups := groupSpans(refinement.Spans("一致", []string{"伊智", "益智", "意志"}))
	b := []byte(`{"answers":{"s1":{"type":"choice","choice":"c1","probabilities":{"keep":0.25,"c1":0.25,"c2":0.25,"c3":0.25}}}}`)
	d, err := answers(b, []string{"s1"}, groups)
	if err != nil {
		t.Fatal(err)
	}
	res := apply("一致", d)
	if res.Text != "一致" || len(res.Keep) != 0 || len(res.Replaced) != 0 {
		t.Fatalf("%+v", res)
	}
}

func TestAnswersRejectInvalidOrIncompleteChoices(t *testing.T) {
	groups := groupSpans(refinement.Spans("一致", []string{"伊智"}))
	for _, body := range []string{
		`{}`, `{"answers":{"s1":{"type":"noul","noul":0.9}}}`,
		`{"answers":{"s1":{"type":"choice","choice":"c1","probabilities":{"c1":0.9}}}}`,
		`{"answers":{"s1":{"type":"choice","choice":"c1","probabilities":{"c1":0.9,"keep":0.9}}}}`,
		`{"answers":{"s1":{"type":"choice","choice":"c1","probabilities":{"c1":0.1,"keep":0.9}}}}`,
		`{"answers":{"s1":{"type":"choice","choice":"c2","probabilities":{"c2":0.9,"keep":0.1}}}}`,
		`{"answers":{"s1":{"type":"choice","choice":"c1","probabilities":{"c1":1.1,"keep":-0.1}}}}`,
		`{"answers":{"s1":{"type":"choice","choice":"c1","probabilities":{"c1":1,"keep":null}}}}`,
	} {
		if _, err := answers([]byte(body), []string{"s1"}, groups); !errors.Is(err, ErrResponse) {
			t.Errorf("accepted %s", body)
		}
	}
}

func TestKnownTermsAndLongTextSendNothing(t *testing.T) {
	cfg, seen := serve(t, func(string, string) float64 { return 1 })
	text := "保持一致性"
	res, err := Check(context.Background(), cfg, text, []string{"一致性", "伊智"}, "test-key")
	if err != nil || res.Text != text || len(*seen) != 0 {
		t.Fatalf("%+v %v", res, err)
	}
	text = strings.Repeat("前", maxTextRunes) + "一致"
	res, err = Check(context.Background(), cfg, text, []string{"伊智"}, "test-key")
	if !errors.Is(err, ErrTooLong) || res.Text != text || len(*seen) != 0 {
		t.Fatalf("long text: %v", err)
	}
}

func TestBoundariesRetainFragmentedAndPartialNames(t *testing.T) {
	text := "打开施玉宝后台，我们一心一意，🙂一致科技"
	tokens, err := segmentation.Cut(text)
	if err != nil {
		t.Fatal(err)
	}
	spans := refinement.Spans(text, []string{"私域宝", "心仪", "伊智"})
	found := map[string]string{}
	for _, s := range spans {
		found[s.Heard] = boundary(s, tokens)
	}
	if found["施玉宝"] == "" || found["一致"] != "word" || found["心一"] != "partial" {
		t.Fatalf("boundaries: %v tokens: %+v", found, tokens)
	}
}

func TestUnjudgedOccurrenceDoesNotInheritKeep(t *testing.T) {
	text := strings.Repeat("意见一致。", maxGroups) + "联系一致科技"
	cfg, _ := serve(t, func(string, string) float64 { return 0.01 })
	res, err := Check(context.Background(), cfg, text, []string{"伊智"}, "test-key")
	if err != nil || res.Text != text || len(res.Decisions) != maxGroups || len(res.Keep) != 0 {
		t.Fatalf("unjudged occurrence protected: %+v %v", res, err)
	}
}

func TestCancelledCheckSendsNothing(t *testing.T) {
	cfg, seen := serve(t, func(string, string) float64 { return 1 })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err := Check(ctx, cfg, TestText, TestTerms, "test-key")
	if !errors.Is(err, context.Canceled) || res.Text != TestText || len(*seen) != 0 {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestGroupsAreBoundedWithoutDroppingAlternatives(t *testing.T) {
	var spans []refinement.Span
	for i := 0; i < maxOptions+1; i++ {
		spans = append(spans, refinement.Span{Candidate: refinement.Candidate{Term: fmt.Sprint(i)}, Start: 0, End: 2})
	}
	spans = append(spans, refinement.Span{Start: 3, End: 5})
	g := groupSpans(spans)
	if len(g) != 1 || g[0].Start != 3 {
		t.Fatalf("oversized group truncated: %+v", g)
	}
	spans = nil
	for i := 0; i < maxGroups+3; i++ {
		spans = append(spans, refinement.Span{Start: 3 * i, End: 3*i + 2})
	}
	if len(groupSpans(spans)) != maxGroups {
		t.Fatal("unbounded questions")
	}
}
