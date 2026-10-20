// Package jev judges the vocabulary corrections of a dictation with Jev,
// TypeSafe's System One model: for each span that sounds like a vocabulary
// term, the probability that the speaker said the term. Code, not a
// language model, then replaces the likely ones and keeps the unlikely.
package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/vfasky/my-ai-audio/gui/internal/refinement"
)

// Config is the setting of the vocabulary check; the zero value is off.
// Endpoint is the System One URL of TypeSafe or of a provider that serves
// Jev, Model the model there; Normalize fills TypeSafe's.
type Config struct {
	Enabled  bool   `json:"enabled,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Model    string `json:"model,omitempty"`
}

// TypeSafe's own API.
const (
	DefaultEndpoint = "https://api.typesafe.ai/v1/systemone"
	DefaultModel    = "jev-latest"
)

// systemOne is the path of the API, added to an endpoint given as a host.
const systemOne = "/v1/systemone"

// Normalize trims c and fills the defaults; an endpoint without a path gets
// the System One path.
func Normalize(c Config) Config {
	c.Endpoint, c.Model = strings.TrimSpace(c.Endpoint), strings.TrimSpace(c.Model)
	if c.Endpoint == "" {
		c.Endpoint = DefaultEndpoint
	}
	if u, err := url.Parse(c.Endpoint); err == nil && strings.Trim(u.Path, "/") == "" && u.Host != "" {
		c.Endpoint = strings.TrimRight(c.Endpoint, "/") + systemOne
	}
	if c.Model == "" {
		c.Model = DefaultModel
	}
	return c
}

// URL returns the endpoint of c, or ErrConfig unless it is HTTPS without
// credentials, query or fragment.
func (c Config) URL() (string, error) {
	c = Normalize(c)
	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", ErrConfig
	}
	return c.Endpoint, nil
}

// Host is where c sends the spans.
func (c Config) Host() string {
	if u, err := url.Parse(Normalize(c).Endpoint); err == nil && u.Host != "" {
		return u.Host
	}
	return Normalize(c).Endpoint
}

// KeyAccount is the Keychain account of the API key of c's endpoint, so
// that each provider keeps its own key; it cannot collide with the
// endpoints that key refinement.
func KeyAccount(c Config) string {
	if e := Normalize(c).Endpoint; e != DefaultEndpoint {
		return "vocabulary:jev:" + e
	}
	return "vocabulary:typesafe-jev"
}

const (
	// A span is replaced at Replace or above, and kept, with refinement
	// told not to change it, at Keep or below; refinement still decides
	// the others.
	Replace = 0.7
	Keep    = 0.3
	// Timeout bounds the check of one dictation; past it the dictation
	// goes on without.
	Timeout = 4 * time.Second
	// maxSpans bounds the questions of one request.
	maxSpans = 24
	// contextRunes of the dictation each side of a span go with it.
	contextRunes = 40
)

// transport carries the requests; tests replace it.
var transport http.RoundTripper = http.DefaultTransport

// Why a check failed.
var (
	ErrKey      = errors.New("jev: key unavailable")
	ErrConfig   = errors.New("jev: invalid endpoint")
	ErrResponse = errors.New("jev: invalid response")
)

// StatusError is a reply other than HTTP 200.
type StatusError struct{ Code int }

func (e *StatusError) Error() string { return fmt.Sprintf("jev: HTTP %d", e.Code) }

// Decision is Jev's answer for one span: P that the speaker said Term.
type Decision struct {
	refinement.Span
	P float64
}

// Result is a dictation after the check.
type Result struct {
	// Text has the likely terms in place of what was heard.
	Text string
	// Decisions are every span judged, Replaced those applied.
	Decisions, Replaced []Decision
	// Keep are the heard words that every remaining judgment kept.
	Keep []string
}

// Check asks Jev about the spans of text that sound like a term and
// applies the answers. Without such spans it sends nothing.
func Check(ctx context.Context, c Config, text string, terms []string, key string) (Result, error) {
	spans := refinement.Spans(text, terms)
	if len(spans) == 0 {
		return Result{Text: text}, nil
	}
	endpoint, err := c.URL()
	if err != nil {
		return Result{Text: text}, err
	}
	if key == "" {
		return Result{Text: text}, ErrKey
	}
	if len(spans) > maxSpans {
		spans = spans[:maxSpans]
	}
	body, names := request(Normalize(c).Model, text, spans)
	status, b, err := post(ctx, endpoint, key, body)
	if err != nil {
		return Result{Text: text}, err
	}
	if status != 200 {
		return Result{Text: text}, &StatusError{status}
	}
	probabilities, err := answers(b, names)
	if err != nil {
		return Result{Text: text}, err
	}
	decisions := make([]Decision, len(spans))
	for i, s := range spans {
		decisions[i] = Decision{s, probabilities[i]}
	}
	return apply(text, decisions), nil
}

// question is a Noul: the probability that its instructions hold.
type question struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria"`
}

// request builds the body: each span is a field of the state, with the
// dictation around it marked, and one question names it, so that each
// judgment sees only its span.
func request(model, text string, spans []refinement.Span) ([]byte, []string) {
	runes := []rune(text)
	fields := map[string]any{}
	questions := map[string]question{}
	names := make([]string, len(spans))
	for i, s := range spans {
		name := fmt.Sprintf("s%d", i+1)
		names[i] = name
		from, to := max(0, s.Start-contextRunes), min(len(runes), s.End+contextRunes)
		fields[name] = map[string]string{
			"context": string(runes[from:s.Start]) + "【" + s.Heard + "】" + string(runes[s.End:to]),
			"heard":   s.Heard,
			"term":    s.Term,
			"pinyin":  s.Pinyin,
			"match":   s.Match,
		}
		questions[name] = question{
			Type: "noul",
			Instructions: fmt.Sprintf("Judge only state field spans.%s. Speech recognition wrote the bracketed words of spans.%s.context as spans.%s.heard. "+
				"Did the speaker actually say the vocabulary term spans.%s.term there?", name, name, name, name),
			Criteria: map[string]string{
				"true":  "Writing the term in place of the bracketed words gives what the speaker meant.",
				"false": "The bracketed words as written are what the speaker meant.",
			},
		}
	}
	state := map[string]any{
		"note":  "Dictated Chinese text. The vocabulary lists names and terms the user says often; speech recognition does not know them and writes them as common words of the same or a similar sound (match: same or similar pinyin).",
		"spans": fields,
	}
	b, _ := json.Marshal(map[string]any{"model": model, "state": state, "questions": questions})
	return b, names
}

// answers reads the probability of each named question.
func answers(b []byte, names []string) ([]float64, error) {
	var v struct {
		Answers map[string]struct {
			Type string   `json:"type"`
			Noul *float64 `json:"noul"`
		} `json:"answers"`
	}
	if json.Unmarshal(b, &v) != nil {
		return nil, ErrResponse
	}
	out := make([]float64, len(names))
	for i, n := range names {
		a, ok := v.Answers[n]
		if !ok || a.Type != "noul" || a.Noul == nil || *a.Noul < 0 || *a.Noul > 1 {
			return nil, ErrResponse
		}
		out[i] = *a.Noul
	}
	return out, nil
}

// apply replaces the likely spans, most likely first, skipping any that
// overlaps one already replaced; a heard word is kept when every
// judgment left in the text found it unlikely.
func apply(text string, decisions []Decision) Result {
	res := Result{Text: text, Decisions: decisions}
	order := slices.Clone(decisions)
	slices.SortStableFunc(order, func(a, b Decision) int {
		switch {
		case a.P > b.P:
			return -1
		case a.P < b.P:
			return 1
		}
		return 0
	})
	overlaps := func(d Decision) bool {
		for _, r := range res.Replaced {
			if d.Start < r.End && r.Start < d.End {
				return true
			}
		}
		return false
	}
	for _, d := range order {
		if d.P >= Replace && !overlaps(d) {
			res.Replaced = append(res.Replaced, d)
		}
	}
	slices.SortFunc(res.Replaced, func(a, b Decision) int { return b.Start - a.Start })
	runes := []rune(text)
	for _, d := range res.Replaced {
		runes = slices.Concat(runes[:d.Start], []rune(d.Term), runes[d.End:])
	}
	res.Text = string(runes)
	keep := map[string]bool{}
	for _, d := range decisions {
		if overlaps(d) {
			continue
		}
		if seen, ok := keep[d.Heard]; !ok || seen {
			keep[d.Heard] = d.P <= Keep
		}
	}
	for _, d := range decisions {
		if keep[d.Heard] && !slices.Contains(res.Keep, d.Heard) {
			res.Keep = append(res.Keep, d.Heard)
		}
	}
	return res
}

// post sends a request and returns the status and the reply, at most
// 256 KiB.
func post(ctx context.Context, endpoint, key string, body []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect refused") }}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	const limit = 256 * 1024
	b, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if res.StatusCode == 200 && (err != nil || len(b) > limit) {
		return 0, nil, ErrResponse
	}
	return res.StatusCode, b, nil
}

// TestText and TestTerms are what Test sends: a fixed sample, nothing of
// the user's speech or vocabulary, with one homophone of the term to
// replace (一致科技) and one ordinary word to keep (意见很一致).
const TestText = "下午我们讨论一致科技的发布计划，大家意见很一致"

var TestTerms = []string{"伊智"}

// TestResult is how Jev judged the sample.
type TestResult struct {
	Result
	Elapsed time.Duration
}

// Test checks the endpoint, model and key of c with the sample.
func Test(ctx context.Context, c Config, key string) (TestResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	started := time.Now()
	res, err := Check(ctx, c, TestText, TestTerms, key)
	return TestResult{res, time.Since(started)}, err
}

// Message says in the interface's language why a check failed.
func Message(err error) string {
	var status *StatusError
	switch {
	case errors.Is(err, ErrConfig):
		return "接口地址无效：需要 https:// 开头、不含账号和查询参数的地址，例如 " + DefaultEndpoint + "。"
	case errors.Is(err, ErrKey):
		return "还没有这个接口的 API key：请填写后保存。"
	case errors.As(err, &status) && status.Code == 401:
		return "接口拒绝了 API key（HTTP 401）：请检查 key 是否正确或已失效。"
	case errors.As(err, &status) && status.Code == 403:
		return "这个 API key 无权使用该模型（HTTP 403）。"
	case errors.As(err, &status) && status.Code == 404:
		return "接口地址或模型不存在（HTTP 404）：请检查地址是否以 /v1/systemone 结尾、模型名是否正确。"
	case errors.As(err, &status) && (status.Code == 400 || status.Code == 422):
		return fmt.Sprintf("接口拒绝了请求（HTTP %d）：请检查模型名，或该接口是否兼容 TypeSafe System One。", status.Code)
	case errors.As(err, &status) && (status.Code == 429 || status.Code == 529):
		return fmt.Sprintf("接口暂时繁忙或超出额度（HTTP %d），请稍后再试。", status.Code)
	case errors.As(err, &status):
		return fmt.Sprintf("接口返回 HTTP %d。", status.Code)
	case errors.Is(err, context.DeadlineExceeded):
		return "连接接口超时。"
	case errors.Is(err, ErrResponse):
		return "接口的回复无法识别：它可能不兼容 TypeSafe System One。"
	}
	return "无法连接接口。"
}
