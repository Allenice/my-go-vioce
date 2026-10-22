package asr

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

var doubaoSeed = Choices[5].Config

// doubaoPeer is a server of the binary protocol: it checks the handshake
// headers, then answers every client packet like the bidirectional endpoint.
type doubaoPeer struct {
	headers http.Header
	request map[string]any
	audio   [][]byte
	flags   []byte
	mode    string // "", "error", "eof", "silent"
}

func (p *doubaoPeer) reply(flags byte, text string, gz bool) []byte {
	body, _ := json.Marshal(map[string]any{"result": map[string]any{"text": text}, "audio_info": map[string]any{"duration": 1}})
	compression := byte(0)
	if gz {
		var z bytes.Buffer
		w := gzip.NewWriter(&z)
		_, _ = w.Write(body)
		_ = w.Close()
		body, compression = z.Bytes(), 1
	}
	b := []byte{0x11, doubaoFullServer<<4 | flags, 0x10 | compression, 0}
	if flags&doubaoSequence != 0 {
		b = binary.BigEndian.AppendUint32(b, 1)
	}
	b = binary.BigEndian.AppendUint32(b, uint32(len(body)))
	return append(b, body...)
}

func (p *doubaoPeer) serve(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.headers = r.Header.Clone()
		if r.URL.Path != "/api/v3/sauc/bigmodel" {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.Header.Get("X-Api-Key") == "bad" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		conn, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", accept(r.Header.Get("Sec-WebSocket-Key")))
		_ = rw.Flush()
		send := func(b []byte) { _ = writeFrame(conn, opBinary, b, false) }
		reader := bufio.NewReader(rw)
		first := true
		for {
			_, op, frame, err := readFrame(reader)
			if err != nil || op == opClose {
				return
			}
			if op != opBinary || len(frame) < 8 || frame[0] != 0x11 || int(binary.BigEndian.Uint32(frame[4:])) != len(frame)-8 {
				t.Errorf("malformed client packet: op=%d % x", op, frame[:min(len(frame), 8)])
				return
			}
			kind, flags, payload := frame[1]>>4, frame[1]&0xF, frame[8:]
			if first {
				first = false
				if kind != doubaoFullClient || flags != 0 || frame[2] != 0x10 || json.Unmarshal(payload, &p.request) != nil {
					t.Errorf("first packet must be an uncompressed JSON full client request: % x", frame[:4])
				}
				switch p.mode {
				case "error":
					msg := "invalid resource"
					b := append([]byte{0x11, doubaoError << 4, 0x10, 0}, binary.BigEndian.AppendUint32(binary.BigEndian.AppendUint32(nil, 45000030), uint32(len(msg)))...)
					send(append(b, msg...))
					return
				case "silent":
					continue
				}
				send(p.reply(doubaoSequence, "", false))
				continue
			}
			if kind != doubaoAudio || frame[2] != 0 {
				t.Errorf("audio packet type %d serialization/compression % x", kind, frame[2:4])
			}
			p.audio, p.flags = append(p.audio, append([]byte(nil), payload...)), append(p.flags, flags)
			switch {
			case flags == doubaoLast && p.mode == "eof":
				return
			case flags == doubaoLast:
				send(p.reply(doubaoSequence|doubaoLast, "你好，世界。", true))
			case len(p.audio) == 1:
				send(p.reply(doubaoSequence, "你好", false))
			default:
				send(p.reply(doubaoSequence, "你好，世", true))
			}
		}
	}))
	t.Cleanup(srv.Close)
	old, oldTransport := doubaoEndpoint, transport
	doubaoEndpoint, transport = "ws"+strings.TrimPrefix(srv.URL, "http")+"/api/v3/sauc/bigmodel", http.DefaultTransport
	t.Cleanup(func() { doubaoEndpoint, transport = old, oldTransport })
}

func TestDoubaoStreamsAudioAndReturnsLastReply(t *testing.T) {
	p := &doubaoPeer{}
	p.serve(t)
	s, events := collect(t, doubaoSeed, "test-key")
	if e := next(t, events); e.Type != "ready" {
		t.Fatalf("event %+v", e)
	}
	pcm := []byte(strings.Repeat("\x01\x02", (chunk*2+640)/2))
	s.PCM(pcm)
	s.Finish()
	for _, want := range []Event{{Type: "partial", Text: "你好"}, {Type: "partial", Text: "你好，世"}, {Type: "final", Text: "你好，世界。"}} {
		if e := next(t, events); e.Type != want.Type || e.Text != want.Text {
			t.Fatalf("event %+v, want %+v", e, want)
		}
	}
	h := p.headers
	if h.Get("X-Api-Key") != "test-key" || h.Get("X-Api-Resource-Id") != "volc.seedasr.sauc.duration" || h.Get("Authorization") != "" ||
		!regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-8[0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(h.Get("X-Api-Connect-Id")) {
		t.Fatalf("handshake headers: %v", h)
	}
	audio := p.request["audio"].(map[string]any)
	req := p.request["request"].(map[string]any)
	if audio["format"] != "pcm" || audio["rate"] != 16000.0 || audio["bits"] != 16.0 || audio["channel"] != 1.0 || req["model_name"] != "bigmodel" {
		t.Fatalf("request: %v", p.request)
	}
	var sent []byte
	for _, b := range p.audio[:len(p.audio)-1] {
		sent = append(sent, b...)
	}
	last := p.audio[len(p.audio)-1]
	if string(sent) != string(pcm) || len(last) != chunk || string(last) != string(make([]byte, chunk)) {
		t.Fatalf("PCM changed or last packet is not silence: %d bytes sent, last %d", len(sent), len(last))
	}
	for i, f := range p.flags {
		if want := byte(0); i == len(p.flags)-1 {
			if f != doubaoLast {
				t.Errorf("last packet flags %x", f)
			}
		} else if f != want {
			t.Errorf("packet %d flags %x", i, f)
		}
	}
}

func TestDoubaoEmptyRecordingAndConnectionTest(t *testing.T) {
	p := &doubaoPeer{}
	p.serve(t)
	s, events := collect(t, doubaoSeed, "test-key")
	if e := next(t, events); e.Type != "ready" {
		t.Fatalf("event %+v", e)
	}
	s.Finish()
	if e := next(t, events); e.Type != "final" {
		t.Fatalf("event %+v", e)
	}
	if len(p.audio) != 1 || len(p.audio[0]) != chunk || p.flags[0] != doubaoLast {
		t.Fatalf("an empty recording must still end with one silent last packet: %v", p.flags)
	}
	if res, err := Test(t.Context(), doubaoSeed, "test-key"); err != nil || res.Elapsed <= 0 {
		t.Fatalf("test: %+v %v", res, err)
	}
}

func TestDoubaoFailures(t *testing.T) {
	for name, tc := range map[string]struct {
		mode, key string
		check     func(error) bool
	}{
		"unauthorized": {"", "bad", func(err error) bool { var s *StatusError; return errors.As(err, &s) && s.Code == 401 }},
		"error frame": {"error", "k", func(err error) bool {
			var e *TaskError
			return errors.As(err, &e) && e.Code == "45000030" && e.Message == "invalid resource"
		}},
		"closed before final": {"eof", "k", func(err error) bool { return err != nil }},
	} {
		t.Run(name, func(t *testing.T) {
			p := &doubaoPeer{mode: tc.mode}
			p.serve(t)
			s, events := collect(t, doubaoSeed, tc.key)
			if tc.mode == "eof" {
				if e := next(t, events); e.Type != "ready" {
					t.Fatalf("event %+v", e)
				}
				s.Finish()
			}
			if e := next(t, events); e.Type != "error" || !tc.check(e.Err) {
				t.Fatalf("event %+v", e)
			}
		})
	}
}

func TestDoubaoStartTimeout(t *testing.T) {
	p := &doubaoPeer{mode: "silent"}
	p.serve(t)
	old := StartTimeout
	StartTimeout = 50 * time.Millisecond
	defer func() { StartTimeout = old }()
	_, events := collect(t, doubaoSeed, "k")
	if e := next(t, events); e.Type != "error" || !errors.Is(e.Err, context.DeadlineExceeded) {
		t.Fatalf("event %+v", e)
	}
}

func TestParseDoubao(t *testing.T) {
	p := &doubaoPeer{}
	gz := p.reply(doubaoSequence|doubaoLast, "好", true)
	if r, err := parseDoubao(gz); err != nil || r.Text != "好" || !r.Last {
		t.Fatalf("gzip last: %+v %v", r, err)
	}
	if r, err := parseDoubao(p.reply(0, "无序号", false)); err != nil || r.Text != "无序号" || r.Last {
		t.Fatalf("no sequence: %+v %v", r, err)
	}
	listBody := []byte(`{"result":[{"text":"列表"}]}`)
	list := append(binary.BigEndian.AppendUint32([]byte{0x11, doubaoFullServer << 4, 0x10, 0}, uint32(len(listBody))), listBody...)
	if r, err := parseDoubao(list); err != nil || r.Text != "列表" {
		t.Fatalf("list result: %+v %v", r, err)
	}
	noResult := append(binary.BigEndian.AppendUint32([]byte{0x11, doubaoFullServer << 4, 0x10, 0}, 2), "{}"...)
	if r, err := parseDoubao(noResult); err != nil || r.Text != "" {
		t.Fatalf("no result: %+v %v", r, err)
	}
	good := p.reply(doubaoSequence, "x", false)
	for name, b := range map[string][]byte{
		"empty":           nil,
		"short":           good[:5],
		"truncated":       good[:len(good)-1],
		"extra":           append(append([]byte(nil), good...), 0),
		"version":         append([]byte{0x21}, good[1:]...),
		"unknown type":    append([]byte{good[0], 0xB1}, good[2:]...),
		"compression":     append([]byte{good[0], good[1], 0x1F}, good[3:]...),
		"not json":        append(binary.BigEndian.AppendUint32([]byte{0x11, doubaoFullServer << 4, 0x10, 0}, 3), "abc"...),
		"bad gzip":        append(binary.BigEndian.AppendUint32([]byte{0x11, doubaoFullServer << 4, 0x11, 0}, 3), "abc"...),
		"error truncated": {0x11, doubaoError << 4, 0x10, 0, 0, 0, 0, 1},
	} {
		if _, err := parseDoubao(b); !errors.Is(err, ErrProtocol) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestDoubaoConfigAndCredentialIsolation(t *testing.T) {
	if len(Choices) != 7 {
		t.Fatalf("choices: %d", len(Choices))
	}
	for _, ch := range Choices[5:] {
		c := ch.Config
		if c.Engine != EngineDoubao || !strings.HasPrefix(c.Model, "volc.") || !c.Online() || !c.Streaming() || c.Validate() != nil ||
			Normalize(c) != c || ChoiceOf(c).ID != ch.ID || c.Host() != "openspeech.bytedance.com" || KeyAccount(c) != "asr:doubao" {
			t.Errorf("choice %s: %+v", ch.ID, c)
		}
	}
	if KeyAccount(doubaoSeed) == KeyAccount(fun) || KeyAccount(doubaoSeed) == KeyAccount(Normalize(Config{Engine: EngineOmni})) {
		t.Fatal("providers must not share credentials")
	}
	for _, bad := range []Config{
		{Engine: EngineDoubao},
		{Engine: EngineDoubao, Model: "volc.unknown.sauc.duration"},
		{Engine: EngineDoubao, Model: "volc.bigasr.sauc.duration"},
		{Engine: EngineDoubao, Model: doubaoSeed.Model, Endpoint: "wss://example.com/"},
	} {
		if Normalize(bad) != Choices[0].Config || bad.Validate() == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
}
