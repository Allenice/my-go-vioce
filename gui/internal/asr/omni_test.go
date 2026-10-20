package asr

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A real TLS WebSocket peer checks the wire contract independently of the
// event builders, including Bearer auth and PCM byte preservation.
func serveOmni(t *testing.T, peer func(*conn)) Config {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" || r.URL.Query().Get("model") != defaultOmniModel || r.URL.Path != "/api-ws/v1/realtime" {
			t.Errorf("incorrect realtime request: %s", r.URL.Path)
		}
		nc, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer nc.Close()
		fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", accept(r.Header.Get("Sec-WebSocket-Key")))
		_ = rw.Flush()
		peer(&conn{rw: nc, r: rw.Reader})
	}))
	t.Cleanup(srv.Close)
	old := transport
	transport = srv.Client().Transport
	t.Cleanup(func() { transport = old })
	return Config{Engine: EngineOmni, Model: defaultOmniModel, Endpoint: "wss" + strings.TrimPrefix(srv.URL, "https") + "/api-ws/v1/realtime"}
}

func omniReply(ws *conn, body string) {
	_ = writeFrame(ws.rw, opText, []byte(body), false)
}

func readOmni(t *testing.T, ws *conn) map[string]any {
	t.Helper()
	op, b, err := ws.ReadMessage()
	var v map[string]any
	if err != nil || op != opText || json.Unmarshal(b, &v) != nil {
		t.Errorf("expected JSON text event: opcode=%d err=%v", op, err)
		return nil
	}
	if v["event_id"] == nil {
		t.Error("missing client event_id")
	}
	return v
}

func acceptOmni(t *testing.T, ws *conn) {
	t.Helper()
	omniReply(ws, `{"type":"session.created"}`)
	m := readOmni(t, ws)
	session, ok := m["session"].(map[string]any)
	if !ok || m["type"] != "session.update" {
		t.Error("expected session.update")
		return
	}
	turn, present := session["turn_detection"]
	transcription, _ := session["input_audio_transcription"].(map[string]any)
	if !present || turn != nil || session["input_audio_format"] != "pcm16" || transcription["model"] != "qwen3-asr-flash-realtime" || fmt.Sprint(session["modalities"]) != "[text]" {
		t.Errorf("wrong dictation session: %+v", session)
	}
	omniReply(ws, `{"type":"session.updated"}`)
}

func TestOmniStreamsAndCommitsInputTranscription(t *testing.T) {
	for _, finalFirst := range []bool{false, true} {
		t.Run(fmt.Sprint("finalBeforeCommitAck=", finalFirst), func(t *testing.T) {
			wire := make(chan []byte, 1)
			closed := make(chan struct{})
			c := serveOmni(t, func(ws *conn) {
				defer close(closed)
				acceptOmni(t, ws)
				var audio []byte
				for {
					m := readOmni(t, ws)
					switch m["type"] {
					case "input_audio_buffer.append":
						b, err := base64.StdEncoding.DecodeString(m["audio"].(string))
						if err != nil {
							t.Error(err)
						}
						audio = append(audio, b...)
						if len(audio) == chunk {
							omniReply(ws, `{"type":"conversation.item.input_audio_transcription.delta","item_id":"input","text":"今","stash":"天天汽"}`)
							omniReply(ws, `{"type":"conversation.item.input_audio_transcription.delta","item_id":"input","text":"今天","stash":"天气"}`)
						}
					case "input_audio_buffer.commit":
						wire <- audio
						// Assistant output must never be inserted as dictation.
						omniReply(ws, `{"type":"response.text.delta","delta":"这是模型回答"}`)
						omniReply(ws, `{"type":"response.done","response":{"status":"completed"}}`)
						omniReply(ws, `{"type":"conversation.item.input_audio_transcription.completed","item_id":"unrelated","transcript":"不要插入"}`)
						ack := `{"type":"input_audio_buffer.committed","item_id":"input"}`
						final := `{"type":"conversation.item.input_audio_transcription.completed","item_id":"input","transcript":"今天天气好。"}`
						if finalFirst {
							omniReply(ws, final)
							omniReply(ws, ack)
						} else {
							omniReply(ws, ack)
							omniReply(ws, final)
						}
						if _, _, err := ws.ReadMessage(); err == nil {
							t.Error("client sent an unexpected event after commit")
						}
						return
					default:
						t.Errorf("unexpected event %v", m["type"])
						return
					}
				}
			})
			s, events := collect(t, c, "test-key")
			// Early audio is held until session.updated; preserve a trailing 20 ms.
			pcm := []byte(strings.Repeat("\x01\x02", (chunk+640)/2))
			s.PCM(pcm)
			s.Finish()
			for _, want := range []Event{{Type: "ready"}, {Type: "partial", Text: "今天天汽"}, {Type: "partial", Text: "今天天气"}, {Type: "final", Text: "今天天气好。"}} {
				if e := next(t, events); e.Type != want.Type || e.Text != want.Text {
					t.Fatalf("event %+v, want %+v", e, want)
				}
			}
			if got := <-wire; string(got) != string(pcm) {
				t.Fatalf("PCM changed: %d bytes", len(got))
			}
			select {
			case <-closed:
			case <-time.After(time.Second):
				t.Fatal("connection not closed after final")
			}
		})
	}
}

func TestOmniFailureAndMalformedReplies(t *testing.T) {
	for _, reply := range []string{
		`{"type":"error","error":{"code":"InvalidApiKey","message":"invalid"}}`,
		`{"type":"conversation.item.input_audio_transcription.failed","item_id":"input","error":{"code":"TranscriptionFailed","message":"failed"}}`,
		`{"type":"conversation.item.input_audio_transcription.completed","item_id":"input"}`,
		`{"type":"session.updated"}`,
		`{"type":"input_audio_buffer.committed","item_id":"premature"}`,
		`not JSON`,
	} {
		t.Run(reply, func(t *testing.T) {
			c := serveOmni(t, func(ws *conn) {
				acceptOmni(t, ws)
				omniReply(ws, reply)
			})
			_, events := collect(t, c, "test-key")
			if e := next(t, events); e.Type != "ready" {
				t.Fatalf("ready: %+v", e)
			}
			if e := next(t, events); e.Type != "error" {
				t.Fatalf("failure: %+v", e)
			}
		})
	}
}

func TestOmniEmptyRecordingAndSilentConnectionTest(t *testing.T) {
	for _, testConnection := range []bool{false, true} {
		t.Run(fmt.Sprint(testConnection), func(t *testing.T) {
			wire := make(chan int, 1)
			c := serveOmni(t, func(ws *conn) {
				acceptOmni(t, ws)
				count := 0
				for {
					m := readOmni(t, ws)
					if m["type"] == "input_audio_buffer.commit" {
						wire <- count
						omniReply(ws, `{"type":"input_audio_buffer.committed","item_id":"input"}`)
						omniReply(ws, `{"type":"conversation.item.input_audio_transcription.completed","item_id":"input","transcript":""}`)
						return
					}
					if m["type"] != "input_audio_buffer.append" {
						t.Error("unexpected event")
						return
					}
					b, err := base64.StdEncoding.DecodeString(m["audio"].(string))
					if err != nil || strings.Trim(string(b), "\x00") != "" {
						t.Error("expected silence")
					}
					count += len(b)
				}
			})
			want := chunk
			if testConnection {
				want *= 5
				if _, err := Test(context.Background(), c, "test-key"); err != nil {
					t.Fatal(err)
				}
			} else {
				s, events := collect(t, c, "test-key")
				next(t, events)
				s.Finish()
				if e := next(t, events); e.Type != "final" || e.Text != "" {
					t.Fatalf("empty final: %+v", e)
				}
			}
			if got := <-wire; got != want {
				t.Fatalf("silence %d want %d", got, want)
			}
		})
	}
}

func TestOmniCancellationClosesSocket(t *testing.T) {
	closed := make(chan struct{})
	c := serveOmni(t, func(ws *conn) {
		acceptOmni(t, ws)
		_, _, _ = ws.ReadMessage()
		close(closed)
	})
	s, events := collect(t, c, "test-key")
	next(t, events)
	s.Close()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("cancel left socket open")
	}
	select {
	case e := <-events:
		t.Fatalf("callback after cancel: %+v", e)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestOmniDeadlineWhileWaitingForFinal(t *testing.T) {
	closed := make(chan struct{})
	c := serveOmni(t, func(ws *conn) {
		defer close(closed)
		acceptOmni(t, ws)
		for {
			_, _, err := ws.ReadMessage()
			if err != nil {
				return
			}
			// Keep receiving audio/commit, but never acknowledge or transcribe.
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	events := make(chan Event, 4)
	s := Start(ctx, Options{Config: c, Key: func(context.Context) (string, error) { return "test-key", nil }, OnEvent: func(e Event) { events <- e }})
	defer s.Close()
	if e := next(t, events); e.Type != "ready" {
		t.Fatalf("ready: %+v", e)
	}
	s.PCM(make([]byte, chunk))
	s.Finish()
	if e := next(t, events); e.Type != "error" || !errors.Is(e.Err, context.DeadlineExceeded) {
		t.Fatalf("deadline: %+v", e)
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("deadline left socket open")
	}
}

func TestCloseInterruptsBlockedWebSocketWrite(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	ws := &conn{rw: client}
	writing := make(chan struct{})
	go func() { defer close(writing); _ = ws.WriteText(make([]byte, 3200)) }()
	closed := make(chan struct{})
	go func() { ws.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close blocked behind writer")
	}
	<-writing
}

func TestOmniConfigAndCredentialIsolation(t *testing.T) {
	c := Normalize(Config{Engine: EngineOmni})
	if c.Validate() != nil || !c.Online() || !c.Streaming() || ChoiceOf(c).ID != "aliyun-omni" {
		t.Fatalf("default Omni config: %+v", c)
	}
	if KeyAccount(c) == KeyAccount(fun) || KeyAccount(fun) != "asr:alipay-maas" {
		t.Fatal("providers must not share credentials")
	}
	custom := c
	custom.Endpoint = "wss://workspace.cn-beijing.maas.aliyuncs.com/api-ws/v1/realtime"
	if custom.Validate() != nil || KeyAccount(custom) == KeyAccount(c) || ChoiceOf(custom).Config != custom {
		t.Fatal("custom endpoint and key account")
	}
	for _, endpoint := range []string{"http://example.com", "ws://example.com", "wss://user:password@example.com/", "wss://example.com/?key=x", "wss://example.com/#x", "wss:///missing-host"} {
		bad := c
		bad.Endpoint = endpoint
		if !errors.Is(bad.Validate(), ErrEndpoint) {
			t.Errorf("accepted unsafe endpoint %q", endpoint)
		}
	}
}
