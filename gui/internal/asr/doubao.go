package asr

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

// Volcengine Doubao streaming recognition (bidirectional mode). The resource
// ID picks the billing plan; Config.Model holds one.
const (
	doubaoSeedDuration   = "volc.seedasr.sauc.duration"
	doubaoSeedConcurrent = "volc.seedasr.sauc.concurrent"
)

// DoubaoHost receives the audio of Doubao sessions.
const DoubaoHost = "openspeech.bytedance.com"

// doubaoEndpoint is the bidirectional streaming socket; tests replace it.
var doubaoEndpoint = "wss://" + DoubaoHost + "/api/v3/sauc/bigmodel"

// Message types and flags of the binary protocol; integers are big-endian.
const (
	doubaoFullClient = 0x1
	doubaoAudio      = 0x2
	doubaoFullServer = 0x9
	doubaoError      = 0xF
	doubaoLast       = 0x2 // flag of the last client packet; also set on the final reply
	doubaoSequence   = 0x1 // flag: a 4-byte sequence follows the header
)

// doubaoFrame is a message with the 4-byte header (version 1, header size 4);
// the full client request is JSON, audio is raw PCM, neither is compressed.
func doubaoFrame(kind, flags byte, payload []byte) []byte {
	serialization := byte(0x00)
	if kind == doubaoFullClient {
		serialization = 0x10
	}
	b := []byte{0x11, kind<<4 | flags, serialization, 0}
	b = binary.BigEndian.AppendUint32(b, uint32(len(payload)))
	return append(b, payload...)
}

func doubaoRequest() []byte {
	b, _ := json.Marshal(map[string]any{
		"audio": map[string]any{"format": "pcm", "codec": "raw", "rate": 16000, "bits": 16, "channel": 1},
		"request": map[string]any{
			"model_name": "bigmodel", "enable_itn": true, "enable_punc": true,
		},
	})
	return doubaoFrame(doubaoFullClient, 0, b)
}

func doubaoHeader(key, resource string) http.Header {
	h := http.Header{}
	h.Set("X-Api-Key", key)
	h.Set("X-Api-Resource-Id", resource)
	h.Set("X-Api-Connect-Id", newUUID())
	return h
}

func newUUID() string {
	h := newTaskID()
	return h[:8] + "-" + h[8:12] + "-4" + h[13:16] + "-8" + h[17:20] + "-" + h[20:]
}

// doubaoReply is one server message: the transcript so far and whether it
// is the final reply.
type doubaoReply struct {
	Text string
	Last bool
}

// parseDoubao reads a server message; an error message becomes a TaskError.
func parseDoubao(b []byte) (doubaoReply, error) {
	if len(b) < 4 || b[0]>>4 != 1 || b[0]&0xF != 1 {
		return doubaoReply{}, ErrProtocol
	}
	kind, flags, compression := b[1]>>4, b[1]&0xF, b[2]&0xF
	body := b[4:]
	if kind == doubaoError {
		if len(body) < 8 {
			return doubaoReply{}, ErrProtocol
		}
		code, n := binary.BigEndian.Uint32(body), binary.BigEndian.Uint32(body[4:])
		if uint64(n) != uint64(len(body)-8) {
			return doubaoReply{}, ErrProtocol
		}
		return doubaoReply{}, &TaskError{Code: strconv.FormatUint(uint64(code), 10), Message: string(body[8:])}
	}
	if kind != doubaoFullServer {
		return doubaoReply{}, ErrProtocol
	}
	if flags&doubaoSequence != 0 {
		if len(body) < 4 {
			return doubaoReply{}, ErrProtocol
		}
		body = body[4:]
	}
	if len(body) < 4 || uint64(binary.BigEndian.Uint32(body)) != uint64(len(body)-4) {
		return doubaoReply{}, ErrProtocol
	}
	payload := body[4:]
	switch compression {
	case 0:
	case 1:
		zr, err := gzip.NewReader(bytes.NewReader(payload))
		if err != nil {
			return doubaoReply{}, ErrProtocol
		}
		if payload, err = io.ReadAll(io.LimitReader(zr, maxMessage+1)); err != nil || len(payload) > maxMessage {
			return doubaoReply{}, ErrProtocol
		}
	default:
		return doubaoReply{}, ErrProtocol
	}
	var m struct {
		Result json.RawMessage `json:"result"`
	}
	if json.Unmarshal(payload, &m) != nil {
		return doubaoReply{}, ErrProtocol
	}
	r := doubaoReply{Last: flags&doubaoLast != 0}
	if len(m.Result) == 0 || string(m.Result) == "null" {
		return r, nil
	}
	// The result is an object; older documentation described a list.
	var one struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(m.Result, &one) != nil {
		var list []struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(m.Result, &list) != nil {
			return doubaoReply{}, ErrProtocol
		}
		if len(list) > 0 {
			one.Text = list[0].Text
		}
	}
	r.Text = one.Text
	return r, nil
}

// recognizeDoubao sends the request, streams PCM once the server answers it,
// and returns the text of the reply flagged as the last.
func (s *Session) recognizeDoubao(key string, start *time.Timer) (string, error) {
	ws, err := dial(s.ctx, doubaoEndpoint, doubaoHeader(key, s.opts.Config.Model))
	if err != nil {
		return "", err
	}
	go func() { <-s.ctx.Done(); ws.Close() }()
	if err := ws.WriteBinary(doubaoRequest()); err != nil {
		return "", err
	}
	ready, partial := false, ""
	var submitted atomic.Bool
	for {
		op, b, err := ws.ReadMessage()
		if err != nil {
			if s.ctx.Err() != nil {
				return "", s.ctx.Err()
			}
			return "", err
		}
		if op != opBinary {
			continue
		}
		r, err := parseDoubao(b)
		if err != nil {
			return "", err
		}
		if !ready {
			if r.Last || !start.Stop() {
				return "", ErrProtocol
			}
			ready = true
			go s.sendDoubao(ws, &submitted)
			s.emit(Event{Type: "ready"})
			continue
		}
		if r.Last {
			if !submitted.Load() {
				return "", ErrProtocol
			}
			return r.Text, nil
		}
		if r.Text != partial {
			partial = r.Text
			s.emit(Event{Type: "partial", Text: r.Text})
		}
	}
}

// sendDoubao writes the queued audio as audio-only requests. The last packet
// carries 100 ms of silence, so the service never sees an empty one, even for
// a recording shorter than a packet.
func (s *Session) sendDoubao(ws *conn, submitted *atomic.Bool) {
	for {
		select {
		case <-s.ctx.Done():
			return
		case b := <-s.out:
			flags, last := byte(0), len(b) == 0
			if last {
				flags, b = doubaoLast, make([]byte, chunk)
				// Bound a missing final reply independently of the UI owner.
				finalTimer := time.AfterFunc(15*time.Second, func() { s.fail(context.DeadlineExceeded) })
				go func() { <-s.ctx.Done(); finalTimer.Stop() }()
				submitted.Store(true)
			}
			if err := ws.WriteBinary(doubaoFrame(doubaoAudio, flags, b)); err != nil {
				s.fail(err)
				return
			}
			if last {
				return
			}
		}
	}
}
