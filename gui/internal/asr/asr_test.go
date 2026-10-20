package asr

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeServer answers the duplex task protocol: task-started, a partial for
// the first chunk, and two sentences for finish-task; or a refused upgrade,
// a failed task, or silence.
type fakeServer struct {
	key, model string
	chunks     []int
	failTask   bool
	silent     bool
	status     int
}

func (f *fakeServer) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if f.status != 0 {
			w.WriteHeader(f.status)
			_, _ = w.Write([]byte(`{"error":{"code":"model_not_allowed"}}`))
			return
		}
		f.key, f.model = r.Header.Get("Authorization"), r.URL.Query().Get("model")
		conn, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", accept(r.Header.Get("Sec-WebSocket-Key")))
		_ = rw.Flush()
		send := func(v any) {
			b, _ := json.Marshal(v)
			_ = writeFrame(conn, opText, b, false)
		}
		sentence := func(id int, text string, end bool) {
			send(map[string]any{"header": map[string]any{"event": "result-generated"}, "payload": map[string]any{"output": map[string]any{"sentence": map[string]any{"sentence_id": id, "text": text, "sentence_end": end}}}})
		}
		reader := bufio.NewReader(rw)
		for {
			_, op, payload, err := readFrame(reader)
			if err != nil {
				return
			}
			switch op {
			case opBinary:
				f.chunks = append(f.chunks, len(payload))
				if len(f.chunks) == 1 {
					sentence(1, "你好", false)
				}
			case opText:
				var m struct {
					Header  struct{ Action string } `json:"header"`
					Payload struct{ Model string }  `json:"payload"`
				}
				_ = json.Unmarshal(payload, &m)
				switch m.Header.Action {
				case "run-task":
					if f.silent {
						continue
					}
					if f.failTask {
						send(map[string]any{"header": map[string]any{"event": "task-failed", "error_code": "InvalidParameter", "error_message": "bad"}})
						continue
					}
					send(map[string]any{"header": map[string]any{"event": "task-started"}})
				case "finish-task":
					sentence(1, "你好，世界。", true)
					sentence(2, "第二句。", true)
					send(map[string]any{"header": map[string]any{"event": "task-finished"}})
				}
			case opClose:
				return
			}
		}
	}
}

func serve(t *testing.T, f *fakeServer) {
	t.Helper()
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	old, oldTransport := endpoint, transport
	endpoint, transport = "ws"+strings.TrimPrefix(srv.URL, "http")+"/api-ws/v1/inference", http.DefaultTransport
	t.Cleanup(func() { endpoint, transport = old, oldTransport })
}

func collect(t *testing.T, c Config, key string) (*Session, chan Event) {
	events := make(chan Event, 64)
	s := Start(context.Background(), Options{Config: c, Key: func(context.Context) (string, error) {
		if key == "" {
			return "", errors.New("missing")
		}
		return key, nil
	}, OnEvent: func(e Event) { events <- e }})
	t.Cleanup(s.Close)
	return s, events
}

func next(t *testing.T, events chan Event) Event {
	t.Helper()
	select {
	case e := <-events:
		return e
	case <-time.After(5 * time.Second):
		t.Fatal("no event")
	}
	return Event{}
}

var fun = Choices[1].Config

func TestSessionStreamsAudioAndJoinsSentences(t *testing.T) {
	f := &fakeServer{}
	serve(t, f)
	s, events := collect(t, fun, "k")
	if e := next(t, events); e.Type != "ready" {
		t.Fatalf("first event %+v", e)
	}
	if f.key != "Bearer k" || f.model != "fun-asr-realtime" {
		t.Fatalf("request %q %q", f.key, f.model)
	}
	// 20 ms frames go out as 100 ms chunks; Finish sends the rest.
	for i := 0; i < 6; i++ {
		s.PCM(make([]byte, 640))
	}
	if e := next(t, events); e.Type != "partial" || e.Text != "你好" {
		t.Fatalf("partial %+v", e)
	}
	s.Finish()
	var last Event
	for last.Type != "final" {
		last = next(t, events)
		if last.Type == "error" {
			t.Fatal(last.Err)
		}
	}
	if last.Text != "你好，世界。第二句。" {
		t.Fatalf("final %q", last.Text)
	}
	if fmt.Sprint(f.chunks) != "[3200 640]" {
		t.Fatalf("chunks %v", f.chunks)
	}
}

func TestSessionErrors(t *testing.T) {
	f := &fakeServer{failTask: true}
	serve(t, f)
	_, events := collect(t, fun, "k")
	var task *TaskError
	if e := next(t, events); e.Type != "error" || !errors.As(e.Err, &task) || task.Code != "InvalidParameter" {
		t.Fatalf("task failure %+v", e)
	}
	_, events = collect(t, fun, "")
	if e := next(t, events); !errors.Is(e.Err, ErrKey) {
		t.Fatalf("no key %+v", e)
	}
	_, events = collect(t, Choices[0].Config, "k")
	if e := next(t, events); !errors.Is(e.Err, ErrConfig) {
		t.Fatalf("local %+v", e)
	}
	f.failTask, f.status = false, 403
	_, events = collect(t, fun, "k")
	var status *StatusError
	if e := next(t, events); !errors.As(e.Err, &status) || status.Code != 403 || status.Reason != "model_not_allowed" {
		t.Fatalf("refused %+v", e)
	}
}

func TestSessionCloseIsSilent(t *testing.T) {
	serve(t, &fakeServer{silent: true})
	s, events := collect(t, fun, "k")
	time.Sleep(50 * time.Millisecond)
	s.Close()
	select {
	case e := <-events:
		t.Fatalf("event after close %+v", e)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestTestSendsSilence(t *testing.T) {
	f := &fakeServer{}
	serve(t, f)
	if _, err := Test(context.Background(), fun, "k"); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(f.chunks) != "[3200 3200 3200 3200 3200]" {
		t.Fatalf("chunks %v", f.chunks)
	}
}

func TestTranscriptKeepsOrderAndSpaces(t *testing.T) {
	var tr Transcript
	tr.Add(Sentence{ID: 2, Text: "Second one."})
	tr.Add(Sentence{ID: 1, Text: "First."})
	tr.Add(Sentence{ID: 3, Text: " 第三句。"})
	tr.Add(Sentence{ID: 4, Text: ""})
	if got := tr.Text(); got != "First. Second one.第三句。" {
		t.Fatalf("%q", got)
	}
}

func TestChoicesAndMessages(t *testing.T) {
	if Normalize(Config{}) != Choices[0].Config || Normalize(Config{Engine: "alipay", Model: "x"}) != Choices[0].Config {
		t.Fatal("unknown engines must fall back to local")
	}
	if ChoiceOf(fun).ID != "alipay/fun-asr-realtime" || !fun.Online() || Choices[0].Config.Online() {
		t.Fatal("choice")
	}
	for err, want := range map[error]string{
		ErrKey:                   "没有可用的 API key",
		context.DeadlineExceeded: "超时",
		&StatusError{Code: 401}:  "HTTP 401",
		&StatusError{Code: 403}:  "无权使用",
		&TaskError{Code: "X"}:    "在线识别失败：X",
		errors.New("dial"):       "连接失败",
	} {
		if got := Message(err); !strings.Contains(got, want) {
			t.Errorf("%v: %q", err, got)
		}
	}
}

func TestFramesRoundTrip(t *testing.T) {
	for _, n := range []int{0, 125, 126, 70000} {
		var b strings.Builder
		payload := []byte(strings.Repeat("a", n))
		if err := writeFrame(&b, opBinary, payload, true); err != nil {
			t.Fatal(err)
		}
		fin, op, got, err := readFrame(bufio.NewReader(strings.NewReader(b.String())))
		if err != nil || !fin || op != opBinary || string(got) != string(payload) {
			t.Fatalf("%d: %v %v %d", n, err, fin, len(got))
		}
	}
}

func TestAcceptMatchesRFC6455(t *testing.T) {
	if got := accept("dGhlIHNhbXBsZSBub25jZQ=="); got != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" {
		t.Fatal(got)
	}
}

var flashChoice = Choices[3].Config

// serveFlash answers the whole-recording model with reply, or status.
func serveFlash(t *testing.T, status int, reply string) *struct {
	key, model string
	pcm        []byte
} {
	got := &struct {
		key, model string
		pcm        []byte
	}{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
			Input struct {
				Messages []struct {
					Content []struct{ Audio string } `json:"content"`
				} `json:"messages"`
			} `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		got.key, got.model = r.Header.Get("Authorization"), body.Model
		audio := strings.TrimPrefix(body.Input.Messages[0].Content[0].Audio, "data:audio/wav;base64,")
		b, _ := base64.StdEncoding.DecodeString(audio)
		if len(b) < 44 || string(b[:4]) != "RIFF" || string(b[36:40]) != "data" {
			t.Errorf("not a WAV: %d bytes", len(b))
		} else {
			got.pcm = b[44:]
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)
	old, oldTransport := flashEndpoint, transport
	flashEndpoint, transport = srv.URL, http.DefaultTransport
	t.Cleanup(func() { flashEndpoint, transport = old, oldTransport })
	return got
}

func TestWholeRecordingIsSentOnFinish(t *testing.T) {
	got := serveFlash(t, 200, `{"output":{"choices":[{"message":{"content":[{"text":"<|zh|>你好，"},{"text":"世界。 "}]}}]}}`)
	s, events := collect(t, flashChoice, "k")
	if e := next(t, events); e.Type != "ready" {
		t.Fatalf("first event %+v", e)
	}
	for i := 0; i < 7; i++ {
		s.PCM(make([]byte, 640))
	}
	s.Finish()
	e := next(t, events)
	if e.Type != "final" || e.Text != "你好，世界。" {
		t.Fatalf("final %+v", e)
	}
	if got.key != "Bearer k" || got.model != "qwen3-asr-flash" || len(got.pcm) != 7*640 {
		t.Fatalf("request %q %q %d", got.key, got.model, len(got.pcm))
	}
	if flashChoice.Streaming() || !fun.Streaming() || Choices[0].Config.Streaming() {
		t.Fatal("streaming")
	}
}

func TestWholeRecordingErrors(t *testing.T) {
	serveFlash(t, 401, `{"error":{"code":"invalid_api_key"}}`)
	s, events := collect(t, flashChoice, "k")
	next(t, events)
	s.Finish()
	var status *StatusError
	if e := next(t, events); !errors.As(e.Err, &status) || status.Code != 401 || status.Reason != "invalid_api_key" {
		t.Fatalf("refused %+v", e)
	}
	if _, err := flashText([]byte(`{"output":{"choices":[]}}`)); !errors.Is(err, ErrProtocol) {
		t.Fatalf("no choice: %v", err)
	}
}
