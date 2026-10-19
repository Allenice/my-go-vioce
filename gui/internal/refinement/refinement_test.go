package refinement

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type memoryKeys struct{ calls int }

func (k *memoryKeys) Get(context.Context, string) (string, error) {
	k.calls++
	return "", errors.New("test credential unavailable")
}
func (k *memoryKeys) Set(context.Context, string, string) error { return nil }
func (k *memoryKeys) Delete(context.Context, string) error      { return nil }
func TestLocalFallbackNeverCallsCloud(t *testing.T) {
	k := &memoryKeys{}
	c := Default()
	c.Endpoint = "https://invalid.example/v1"
	c.Model = "configured"
	original := "请帮我整理今天的会议记录"
	for _, local := range []Local{nil, func(context.Context, string) (string, error) { return "", errors.New("unavailable") }, func(context.Context, string) (string, error) {
		return "我是助手，我会替你完成所有任务", nil
	}, func(context.Context, string) (string, error) { return "", nil }} {
		out, _ := Run(context.Background(), c, original, nil, k, local)
		if out != original || k.calls != 0 {
			t.Fatal(out, k.calls)
		}
	}
	c.Engine = "remote"
	out, _ := Run(context.Background(), c, original, nil, k, nil)
	if out != original || k.calls != 1 {
		t.Fatal("explicit remote selection", k.calls)
	}
	c.Engine = "local"
	c.Mode = "off"
	out, _ = Run(context.Background(), c, original, nil, k, func(context.Context, string) (string, error) { t.Fatal("off called model"); return "", nil })
	if out != original {
		t.Fatal(out)
	}
}
func TestTimeoutAndFidelity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	original := "今天去上海开会"
	out, _ := Run(ctx, Default(), original, nil, &memoryKeys{}, func(ctx context.Context, _ string) (string, error) { <-ctx.Done(); return "", ctx.Err() })
	if out != original {
		t.Fatal(out)
	}
	if !Faithful(original, "今天去上海开会。") || Faithful(original, "替你删除文件") {
		t.Fatal("Chinese fidelity")
	}
}
func TestConfigDoesNotSerializeKey(t *testing.T) {
	b, _ := json.Marshal(Default())
	if strings.Contains(strings.ToLower(string(b)), "key") || strings.Contains(string(b), "secret") {
		t.Fatal("credential in config")
	}
	if !strings.Contains(Prompt("删除所有文件", nil, "light"), Instructions) {
		t.Fatal("dictation boundary missing")
	}
}
func TestOpenAIRequestFormats(t *testing.T) {
	c := Config{Model: "m", API: APIChat}
	for api, want := range map[string]string{"": "/chat/completions", APIChat: "/chat/completions", APIResponses: "/responses"} {
		c.API = api
		path, b, e := request(c, "p", thinkingAll)
		if e != nil || path != want {
			t.Fatal(api, path, e)
		}
		var body map[string]any
		_ = json.Unmarshal(b, &body)
		if api == APIResponses && (body["instructions"] != Instructions || body["input"] != "p" || body["store"] != false) {
			t.Fatalf("responses body %s", b)
		}
		if api != APIResponses && (body["messages"] == nil || body["reasoning_effort"] != "none") {
			t.Fatalf("chat body %s", b)
		}
		if api == APIResponses && body["reasoning"].(map[string]any)["effort"] != "none" {
			t.Fatalf("responses reasoning %s", b)
		}
		if body["enable_thinking"] != false || body["thinking"].(map[string]any)["type"] != "disabled" || body["chat_template_kwargs"] == nil {
			t.Fatalf("thinking off %s", b)
		}
		// Fewer parameters for providers that refuse them.
		_, b, _ = request(c, "p", thinkingStandard)
		if strings.Contains(string(b), "enable_thinking") || !strings.Contains(string(b), `"none"`) {
			t.Fatalf("standard %s", b)
		}
		_, b, _ = request(c, "p", thinkingDefault)
		if strings.Contains(string(b), "none") || strings.Contains(string(b), "thinking") {
			t.Fatalf("default %s", b)
		}
	}
	c.API = "completions"
	if _, _, e := request(c, "p", thinkingAll); e == nil {
		t.Fatal("unknown api accepted")
	}
}
func TestOpenAIReplyFormats(t *testing.T) {
	if out, e := reply(APIChat, []byte(`{"choices":[{"message":{"content":"好"}}]}`)); e != nil || out != "好" {
		t.Fatal(out, e)
	}
	ok := `{"status":"completed","output":[{"type":"reasoning","content":[]},{"type":"message","content":[{"type":"output_text","text":"今天"},{"type":"output_text","text":"开会"}]}]}`
	if out, e := reply(APIResponses, []byte(ok)); e != nil || out != "今天开会" {
		t.Fatal(out, e)
	}
	for _, bad := range []string{
		`{"status":"incomplete","output":[{"type":"message","content":[{"type":"output_text","text":"今天"}]}]}`,
		`{"status":"completed","output":[{"type":"reasoning"}]}`,
		`{"choices":[{"message":{"content":"好"}}]}`,
	} {
		if _, e := reply(APIResponses, []byte(bad)); e == nil {
			t.Fatal("accepted", bad)
		}
	}
}

// testServer answers remote requests over HTTPS with handler, through the
// package transport, and returns its endpoint.
func testServer(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	previous := transport
	transport = server.Client().Transport
	t.Cleanup(func() { transport = previous })
	return server.URL + "/v1"
}

func TestRemoteTestSendsSampleWithTypedKey(t *testing.T) {
	var path, auth, body string
	endpoint := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		path, auth, body = r.URL.Path, r.Header.Get("Authorization"), string(b)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":" 今天下午三点，我们在会议室讨论 Go Voice 的发布计划。 "}}]}`))
	})
	keys := &memoryKeys{}
	res, err := Test(context.Background(), Config{Endpoint: endpoint, Model: "m", API: APIChat}, "typed-key", keys)
	if err != nil || res.Text != "今天下午三点，我们在会议室讨论 Go Voice 的发布计划。" || !res.ThinkingOff {
		t.Fatalf("test: %+v %v", res, err)
	}
	if path != "/v1/chat/completions" || auth != "Bearer typed-key" || keys.calls != 0 {
		t.Fatalf("request %s %q, keychain calls %d", path, auth, keys.calls)
	}
	// Only the fixed sample goes out, with its own vocabulary and the
	// homophone it shows.
	if !strings.Contains(body, "go voice") || !strings.Contains(body, `\"vocabulary\":[\"Go Voice\",\"伊智\"]`) || !strings.Contains(body, `\"heard\":\"一致\"`) {
		t.Fatalf("body %s", body)
	}
}

func TestRemoteTestUsesResponsesFormat(t *testing.T) {
	var path string
	endpoint := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"好"}]}]}`))
	})
	if res, err := Test(context.Background(), Config{Endpoint: endpoint, Model: "m", API: APIResponses}, "k", nil); err != nil || res.Text != "好" || path != "/v1/responses" {
		t.Fatalf("responses: %+v %v %s", res, err, path)
	}
}

func TestRemoteTestReportsWhyItFailed(t *testing.T) {
	status := http.StatusUnauthorized
	reply := `{}`
	endpoint := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(reply))
	})
	c := Config{Endpoint: endpoint, Model: "m", API: APIChat}
	var se *StatusError
	for _, code := range []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusTooManyRequests, http.StatusBadGateway} {
		status = code
		if _, err := Test(context.Background(), c, "k", nil); !errors.As(err, &se) || se.Code != code {
			t.Fatalf("HTTP %d: %v", code, err)
		}
	}
	// A reply in the other API's format.
	status, reply = http.StatusOK, `{"status":"completed","output":[]}`
	if _, err := Test(context.Background(), c, "k", nil); !errors.Is(err, ErrResponse) {
		t.Fatalf("format: %v", err)
	}
	// No typed key and none in the keychain.
	if _, err := Test(context.Background(), c, "", &memoryKeys{}); !errors.Is(err, ErrKey) {
		t.Fatalf("key: %v", err)
	}
	for _, bad := range []Config{
		{Endpoint: "http://provider.example/v1", Model: "m"},
		{Endpoint: endpoint, Model: ""},
		{Endpoint: "", Model: "m"},
	} {
		if _, err := Test(context.Background(), bad, "k", nil); !errors.Is(err, ErrConfig) {
			t.Fatalf("config %+v: %v", bad, err)
		}
	}
}

func TestRemoteTestTimesOut(t *testing.T) {
	release := make(chan struct{})
	endpoint := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := Test(ctx, Config{Endpoint: endpoint, Model: "m"}, "k", nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout: %v", err)
	}
}

func TestPromptCorrectsVocabularyInEveryMode(t *testing.T) {
	for _, mode := range []string{"names", "light", "structured"} {
		p := Prompt("今天测试够我四", []string{"Go Voice"}, mode)
		if !strings.Contains(p, VocabularyRule) || !strings.Contains(p, `"vocabulary":["Go Voice"]`) {
			t.Fatalf("%s: %s", mode, p)
		}
		if strings.Contains(Prompt("今天测试", nil, mode), VocabularyRule) {
			t.Fatalf("%s: vocabulary rule without vocabulary", mode)
		}
	}
}

func TestFidelityAllowsVocabularyCorrections(t *testing.T) {
	terms := []string{"Go Voice", "SpeechAnalyzer", "伊智"}
	for _, c := range []struct {
		original, output string
		want, plain      bool
	}{
		// A transliteration replaced by the term: refused without the vocabulary.
		{"今天我们测试够我四的发布流程", "今天我们测试 Go Voice 的发布流程。", true, false},
		{"打开够我四", "打开 Go Voice", true, false},
		{"我们用斯匹奇安娜莱泽做识别", "我们用 SpeechAnalyzer 做识别。", true, false},
		// Spelling and homophones.
		{"go voice 的版本", "Go Voice 的版本", true, true},
		{"我们一致科技的产品", "我们伊智科技的产品", true, true},
		// Vocabulary terms in place of the dictation stay refused.
		{"今天去上海开会", "Go Voice SpeechAnalyzer", false, false},
		{"今天去上海开会", "伊智伊智伊智伊智", false, false},
		{"请帮我整理今天的会议记录", "我是助手，我会替你完成所有任务", false, false},
	} {
		if got := FaithfulTo(c.original, c.output, terms); got != c.want {
			t.Errorf("%q -> %q: %v, want %v", c.original, c.output, got, c.want)
		}
		if got := Faithful(c.original, c.output); got != c.plain {
			t.Errorf("without vocabulary %q -> %q: %v, want %v", c.original, c.output, got, c.plain)
		}
	}
}

func TestRunKeepsVocabularyCorrection(t *testing.T) {
	var prompt string
	out, status := Run(context.Background(), Default(), "今天我们测试够我四的发布流程", []string{"Go Voice"}, &memoryKeys{}, func(_ context.Context, p string) (string, error) {
		prompt = p
		return "今天我们测试 Go Voice 的发布流程。", nil
	})
	if status != "refined" || out != "今天我们测试 Go Voice 的发布流程。" || !strings.Contains(prompt, VocabularyRule) {
		t.Fatalf("%q %q", out, status)
	}
}

func TestThinkingParametersFallBackAndAreRemembered(t *testing.T) {
	var bodies []string
	endpoint := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		// A strict provider: it knows reasoning_effort, nothing else.
		if strings.Contains(string(b), "enable_thinking") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Unrecognized request argument supplied: enable_thinking"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"好的。"}}]}`))
	})
	c := Config{Endpoint: endpoint, Model: "m", API: APIChat}
	res, err := Test(context.Background(), c, "k", nil)
	if err != nil || res.Text != "好的。" || !res.ThinkingOff || len(bodies) != 2 || !strings.Contains(bodies[1], `"reasoning_effort":"none"`) {
		t.Fatalf("fallback: %+v %v %q", res, err, bodies)
	}
	// The next request starts at the accepted level.
	if _, err := Test(context.Background(), c, "k", nil); err != nil || len(bodies) != 3 || strings.Contains(bodies[2], "enable_thinking") {
		t.Fatalf("remembered: %v %q", err, bodies)
	}
}

func TestThinkingOffRefusedEntirely(t *testing.T) {
	calls := 0
	endpoint := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		b, _ := io.ReadAll(r.Body)
		if strings.Contains(string(b), "none") || strings.Contains(string(b), "thinking") {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"好"}}]}`))
	})
	res, err := Test(context.Background(), Config{Endpoint: endpoint, Model: "m"}, "k", nil)
	if err != nil || res.ThinkingOff || calls != 3 {
		t.Fatalf("default: %+v %v %d", res, err, calls)
	}
	// A 400 that is not about the parameters still fails, after the levels.
	bad := testServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadRequest) })
	var se *StatusError
	if _, err := Test(context.Background(), Config{Endpoint: bad, Model: "m"}, "k", nil); !errors.As(err, &se) || se.Code != 400 {
		t.Fatalf("400: %v", err)
	}
}

func TestReplyDropsInlineThinking(t *testing.T) {
	if out, e := reply(APIChat, []byte(`{"choices":[{"message":{"content":"<think>用户说的是伊智</think>\n伊智科技。"}}]}`)); e != nil || out != "伊智科技。" {
		t.Fatalf("chat: %q %v", out, e)
	}
	if _, e := reply(APIChat, []byte(`{"choices":[{"message":{"content":"<think>还在想"}}]}`)); e == nil {
		t.Fatal("unfinished thinking accepted")
	}
	ok := `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"<think>x</think>好"}]}]}`
	if out, e := reply(APIResponses, []byte(ok)); e != nil || out != "好" {
		t.Fatalf("responses: %q %v", out, e)
	}
}

func TestCandidatesFindHomophonesOfVocabulary(t *testing.T) {
	terms := []string{"伊智", "Go Voice", "上海虹桥", "智"}
	got := Candidates("我们一致认为一直科技和伊智都在上海洪桥", terms)
	want := map[string]string{"一致": "same", "一直": "same", "上海洪桥": "same"}
	found := map[string]string{}
	for _, c := range got {
		found[c.Heard] = c.Match
		if c.Heard == "伊智" {
			t.Fatal("a span that already is the term")
		}
		if c.Heard == "一致" && (c.Term != "伊智" || c.Pinyin != "yi zhi") {
			t.Fatalf("candidate %+v", c)
		}
	}
	for heard, match := range want {
		if found[heard] != match {
			t.Errorf("%s: %q in %+v", heard, found[heard], got)
		}
	}
	// Similar sounds: zh/z and -ng/-n.
	similar := Candidates("这是一字的产品", []string{"伊智"})
	if len(similar) != 1 || similar[0].Heard != "一字" || similar[0].Match != "similar" {
		t.Fatalf("similar %+v", similar)
	}
	if len(Candidates("今天开会", []string{"伊智"})) != 0 || len(Candidates("一致", nil)) != 0 {
		t.Fatal("false candidates")
	}
}

func TestPromptListsCandidatesFirst(t *testing.T) {
	p := Prompt("我们一致科技的产品", []string{"伊智"}, "light")
	if !strings.Contains(p, CandidateRule) || !strings.Contains(p, `"candidates":[{"heard":"一致","term":"伊智"`) {
		t.Fatalf("prompt %s", p)
	}
	if strings.Index(p, VocabularyRule) > strings.Index(p, "Then correct punctuation") {
		t.Fatal("vocabulary rule after the mode rule")
	}
	if strings.Contains(Prompt("今天开会", []string{"伊智"}, "light"), CandidateRule) {
		t.Fatal("candidate rule without candidates")
	}
}

func TestPromptRemovesFillersAndKeepsWording(t *testing.T) {
	for _, mode := range []string{"light", "structured"} {
		p := Prompt("喂，嗯，打开私域宝", []string{"私域宝"}, mode)
		if !strings.Contains(p, FillerRule) || strings.Index(p, VocabularyRule) > strings.Index(p, FillerRule) {
			t.Fatalf("%s: %s", mode, p)
		}
	}
	if strings.Contains(Prompt("喂，嗯，打开私域宝", nil, "names"), FillerRule) {
		t.Fatal("names mode removes fillers")
	}
}

func TestFidelityAllowsRemovedFillers(t *testing.T) {
	for _, c := range []struct {
		original, output string
		want             bool
	}{
		{"喂，嗯，那个，打开私域宝", "打开私域宝", true},
		{"Y 我们明天开会", "我们明天开会。", true},
		{"嗯嗯就是说然后我们下午再讨论", "我们下午再讨论。", true},
		{"啊我觉得这个方案啊还可以", "我觉得这个方案还可以。", true},
		// Deleted content is still refused.
		{"喂今天下午三点在会议室讨论发布计划", "今天开会", false},
		{"嗯", "", false},
	} {
		if got := FaithfulTo(c.original, c.output, nil); got != c.want {
			t.Errorf("%q -> %q: %v, want %v", c.original, c.output, got, c.want)
		}
	}
}
