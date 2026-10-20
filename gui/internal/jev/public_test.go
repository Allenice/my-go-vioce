package jev

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/vfasky/my-ai-audio/gui/internal/refinement"
	"github.com/vfasky/my-ai-audio/gui/internal/wordlists"
)

func TestPublicChoiceHasProvenanceAndCanKeepOriginal(t *testing.T) {
	cfg, seen := serve(t, func(string, string) float64 { return 0.02 })
	text := "执行初始话"
	e, err := refinement.PrepareEvidence(context.Background(), text, nil, nil, wordlists.Config{BaseDisabled: true})
	if err != nil {
		t.Fatal(err)
	}
	res, err := CheckEvidence(context.Background(), cfg, text, e, "test-key")
	if err != nil || res.Text != text || len(res.Replaced) != 0 || len(*seen) != 1 {
		t.Fatalf("result: %+v %v", res, err)
	}
	b, _ := json.Marshal((*seen)[0])
	body := string(b)
	if !strings.Contains(body, `"source":"it"`) || !strings.Contains(body, `"term":"初始化"`) || !strings.Contains(body, `"keep"`) || !strings.Contains(body, `"rank":`) {
		t.Fatalf("missing provenance: %s", body)
	}
	if strings.Contains(body, "编译器") || strings.Contains(body, "并发控制") {
		t.Fatal("full public pack sent")
	}
}

func TestPublicChoicesCannotDisplacePersonalQuestions(t *testing.T) {
	var spans []refinement.Span
	for i := 0; i < maxGroups+4; i++ {
		spans = append(spans, refinement.Span{Candidate: refinement.Candidate{Source: "base", Term: fmt.Sprint(i)}, Start: i * 3, End: i*3 + 2})
	}
	spans = append(spans, refinement.Span{Candidate: refinement.Candidate{Term: "伊智"}, Start: 100, End: 102})
	groups := groupSpans(spans)
	if len(groups) != maxGroups || groups[len(groups)-1].Spans[0].Term != "伊智" {
		t.Fatal("public groups crowded out personal name")
	}
	spans = nil
	for i := 0; i < maxOptions; i++ {
		spans = append(spans, refinement.Span{Candidate: refinement.Candidate{Source: "base", Term: fmt.Sprint(i)}, Start: 0, End: 2})
	}
	spans = append(spans, refinement.Span{Candidate: refinement.Candidate{Term: "伊智"}, Start: 0, End: 2})
	groups = groupSpans(spans)
	if len(groups) != 1 || len(groups[0].Spans) != 1 || groups[0].Spans[0].Term != "伊智" {
		t.Fatalf("mixed overflow dropped personal name: %+v", groups)
	}
}
