package refinement

import (
	"context"
	"encoding/json"
	"errors"
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
		path, b, e := request(c, "p")
		if e != nil || path != want {
			t.Fatal(api, path, e)
		}
		var body map[string]any
		_ = json.Unmarshal(b, &body)
		if api == APIResponses && (body["instructions"] != Instructions || body["input"] != "p" || body["store"] != false) {
			t.Fatalf("responses body %s", b)
		}
		if api != APIResponses && body["messages"] == nil {
			t.Fatalf("chat body %s", b)
		}
	}
	c.API = "completions"
	if _, _, e := request(c, "p"); e == nil {
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
