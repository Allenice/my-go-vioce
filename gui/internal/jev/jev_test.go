package jev

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/vfasky/my-ai-audio/gui/internal/refinement"
)

// serve answers like a provider of Jev, with p by the heard words of each
// span, and returns the Config that reaches it.
func serve(t *testing.T, p func(heard, context string) float64) (Config, *[]map[string]any) {
	t.Helper()
	var seen []map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxy/v1/systemone" {
			w.WriteHeader(404)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(401)
			return
		}
		b, _ := io.ReadAll(r.Body)
		var req struct {
			Model string
			State struct {
				Spans map[string]struct {
					Context string
					Options map[string]struct{ Heard, Term string }
				}
			}
			Questions map[string]question
		}
		if err := json.Unmarshal(b, &req); err != nil || req.Model != "jev-1.13" {
			w.WriteHeader(422)
			return
		}
		var body map[string]any
		_ = json.Unmarshal(b, &body)
		seen = append(seen, body)
		out := map[string]any{}
		for name, q := range req.Questions {
			span, exists := req.State.Spans[name]
			if q.Type != "choice" || !exists || !strings.Contains(q.Instructions, "spans."+name) || q.Criteria["keep"] == "" {
				w.WriteHeader(422)
				return
			}
			probabilities := map[string]float64{}
			total := 0.0
			for key, option := range span.Options {
				probabilities[key] = p(option.Heard, span.Context) / float64(len(span.Options))
				total += probabilities[key]
			}
			probabilities["keep"] = 1 - total
			winner := "keep"
			for key, probability := range probabilities {
				if probability > probabilities[winner] {
					winner = key
				}
			}
			out[name] = map[string]any{"type": "choice", "choice": winner, "probabilities": probabilities}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": req.Model, "answers": out})
	}))
	t.Cleanup(srv.Close)
	transport = srv.Client().Transport
	t.Cleanup(func() { transport = http.DefaultTransport })
	return Config{Enabled: true, Endpoint: srv.URL + "/proxy/v1/systemone", Model: "jev-1.13"}, &seen
}

func TestCheckReplacesLikelyAndKeepsUnlikely(t *testing.T) {
	cfg, _ := serve(t, func(heard, context string) float64 {
		if strings.Contains(context, "【一致】科技") {
			return 0.93
		}
		return 0.06
	})
	res, err := Check(context.Background(), cfg, TestText, TestTerms, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "下午我们讨论伊智科技的发布计划，大家意见很一致" {
		t.Fatalf("text %q", res.Text)
	}
	if len(res.Decisions) != 2 || len(res.Replaced) != 1 || !slices.Equal(res.Keep, []string{"一致"}) {
		t.Fatalf("result %+v", res)
	}
}

func TestCheckLeavesTheUncertainToRefinement(t *testing.T) {
	cfg, _ := serve(t, func(string, string) float64 { return 0.5 })
	res, err := Check(context.Background(), cfg, "我们讨论一致科技", []string{"伊智"}, "test-key")
	if err != nil || res.Text != "我们讨论一致科技" || len(res.Replaced) != 0 || len(res.Keep) != 0 {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestCheckSendsNothingWithoutSpans(t *testing.T) {
	cfg, seen := serve(t, func(string, string) float64 { return 1 })
	res, err := Check(context.Background(), cfg, "今天天气不错", []string{"伊智"}, "")
	if err != nil || res.Text != "今天天气不错" || len(*seen) != 0 {
		t.Fatalf("%+v %v %d", res, err, len(*seen))
	}
}

func TestCheckSendsFullDictationButOnlyMatchedTerms(t *testing.T) {
	cfg, seen := serve(t, func(string, string) float64 { return 0 })
	long := strings.Repeat("前", 60) + "一致科技" + strings.Repeat("后", 60)
	if _, err := Check(context.Background(), cfg, long, []string{"伊智", "私域宝", "Go Voice"}, "test-key"); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal((*seen)[0])
	sent := string(b)
	if strings.Contains(sent, "私域宝") || strings.Contains(sent, "Go Voice") || !strings.Contains(sent, long) || !strings.Contains(sent, `"tokens":`) {
		t.Fatalf("missing full context/segmentation or unrelated terms sent: %s", sent)
	}
}

func TestApplyPrefersTheLikelierOfOverlappingSpans(t *testing.T) {
	text := "打开施玉宝后台"
	spans := refinement.Spans(text, []string{"私域宝", "石玉"})
	var decisions []Decision
	for _, s := range spans {
		p := 0.8
		if s.Term == "私域宝" {
			p = 0.95
		}
		decisions = append(decisions, Decision{Span: s, P: p})
	}
	if len(decisions) != 2 {
		t.Fatalf("spans %+v", spans)
	}
	if res := apply(text, decisions); res.Text != "打开私域宝后台" {
		t.Fatalf("text %q", res.Text)
	}
}

func TestCheckFailuresKeepTheText(t *testing.T) {
	cfg, _ := serve(t, func(string, string) float64 { return 2 })
	res, err := Check(context.Background(), cfg, TestText, TestTerms, "wrong")
	var status *StatusError
	if !errors.As(err, &status) || status.Code != 401 || res.Text != TestText {
		t.Fatalf("%+v %v", res, err)
	}
	if _, err := Check(context.Background(), cfg, TestText, TestTerms, "test-key"); !errors.Is(err, ErrResponse) {
		t.Fatalf("out of range: %v", err)
	}
	if _, err := Check(context.Background(), cfg, TestText, TestTerms, ""); !errors.Is(err, ErrKey) {
		t.Fatalf("no key: %v", err)
	}
	for _, e := range []error{ErrKey, &StatusError{401}, &StatusError{429}, context.DeadlineExceeded, errors.New("dial")} {
		if Message(e) == "" {
			t.Fatalf("no message for %v", e)
		}
	}
}

func TestConfigDefaultsAndEndpoints(t *testing.T) {
	if c := Normalize(Config{}); c.Endpoint != DefaultEndpoint || c.Model != DefaultModel {
		t.Fatalf("defaults %+v", c)
	}
	if c := Normalize(Config{Endpoint: " https://maas-api.antdigital.com/ ", Model: " jev-1.13 "}); c.Endpoint != "https://maas-api.antdigital.com/v1/systemone" || c.Model != "jev-1.13" {
		t.Fatalf("host only %+v", c)
	}
	if u, err := (Config{Endpoint: "https://maas-api.antdigital.com/v1/systemone"}).URL(); err != nil || u != "https://maas-api.antdigital.com/v1/systemone" {
		t.Fatalf("url %q %v", u, err)
	}
	for _, e := range []string{"http://maas-api.antdigital.com/v1/systemone", "https://user:pw@x.example/v1/systemone", "https://x.example/v1/systemone?key=1", "maas-api.antdigital.com"} {
		if _, err := (Config{Endpoint: e}).URL(); !errors.Is(err, ErrConfig) {
			t.Errorf("%s accepted", e)
		}
	}
	if KeyAccount(Config{}) != "vocabulary:typesafe-jev" || KeyAccount(Config{Endpoint: "https://maas-api.antdigital.com/v1/systemone"}) != "vocabulary:jev:https://maas-api.antdigital.com/v1/systemone" {
		t.Fatal("key accounts")
	}
	if (Config{Endpoint: "https://maas-api.antdigital.com/v1/systemone"}).Host() != "maas-api.antdigital.com" {
		t.Fatal("host")
	}
	if _, err := Check(context.Background(), Config{Endpoint: "http://x.example"}, TestText, TestTerms, "k"); !errors.Is(err, ErrConfig) {
		t.Fatalf("plain http sent: %v", err)
	}
}
