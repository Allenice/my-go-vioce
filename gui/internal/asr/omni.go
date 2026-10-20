package asr

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

// Omni uses the Realtime event protocol, not DashScope run-task. Dictation
// uses Manual mode: append PCM while recording, commit on release, and consume
// only input transcription. response.create would generate an assistant reply.
func omniEvent(kind string, fields map[string]any) []byte {
	if fields == nil {
		fields = map[string]any{}
	}
	fields["event_id"], fields["type"] = "event_"+newTaskID(), kind
	b, _ := json.Marshal(fields)
	return b
}

func omniUpdate() []byte {
	return omniEvent("session.update", map[string]any{"session": map[string]any{
		"modalities":                []string{"text"},
		"input_audio_format":        "pcm16",
		"input_audio_transcription": map[string]any{"model": "qwen3-asr-flash-realtime"},
		"turn_detection":            nil,
	}})
}

type omniServerEvent struct {
	Type       string  `json:"type"`
	ItemID     string  `json:"item_id"`
	Text       *string `json:"text"`
	Stash      string  `json:"stash"`
	Delta      *string `json:"delta"`
	Transcript *string `json:"transcript"`
	Error      struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (s *Session) recognizeOmni(key string, start *time.Timer) (string, error) {
	c := s.opts.Config
	u, _ := url.Parse(c.ServerURL()) // Config.Validate already checked this.
	q := u.Query()
	q.Set("model", c.Model)
	u.RawQuery = q.Encode()
	ws, err := dial(s.ctx, u.String(), key)
	if err != nil {
		return "", err
	}
	go func() { <-s.ctx.Done(); ws.Close() }()
	created, ready := false, false
	var submitted atomic.Bool
	committed := ""
	// Transcription events can precede the commit acknowledgement. Retain
	// them by item ID; never accept an unrelated item as the final dictation.
	partials := map[string]string{}
	finals := map[string]string{}
	for {
		op, b, err := ws.ReadMessage()
		if err != nil {
			if s.ctx.Err() != nil {
				return "", s.ctx.Err()
			}
			return "", err
		}
		if op != opText {
			continue
		}
		var e omniServerEvent
		if json.Unmarshal(b, &e) != nil || e.Type == "" {
			return "", ErrProtocol
		}
		switch e.Type {
		case "session.created":
			if created {
				return "", ErrProtocol
			}
			created = true
			if err := ws.WriteText(omniUpdate()); err != nil {
				return "", err
			}
		case "session.updated":
			if !created || ready {
				return "", ErrProtocol
			}
			if !start.Stop() {
				return "", context.DeadlineExceeded
			}
			ready = true
			go s.sendOmni(ws, &submitted)
			s.emit(Event{Type: "ready"})
		case "input_audio_buffer.committed":
			if !ready || !submitted.Load() || e.ItemID == "" || committed != "" {
				return "", ErrProtocol
			}
			committed = e.ItemID
		case "conversation.item.input_audio_transcription.delta":
			if !ready || e.ItemID == "" || (e.Text == nil && e.Delta == nil) {
				return "", ErrProtocol
			}
			text := partials[e.ItemID]
			if e.Text != nil {
				// text + stash is a replacement preview, not an append delta.
				text = *e.Text + e.Stash
			} else {
				text += *e.Delta
			}
			if len(text) > maxMessage {
				return "", ErrProtocol
			}
			if text != partials[e.ItemID] && (committed == "" || committed == e.ItemID) {
				s.emit(Event{Type: "partial", Text: text})
			}
			partials[e.ItemID] = text
		case "conversation.item.input_audio_transcription.completed":
			if !ready || e.ItemID == "" || e.Transcript == nil {
				return "", ErrProtocol
			}
			finals[e.ItemID] = strings.TrimSpace(*e.Transcript)
		case "error", "conversation.item.input_audio_transcription.failed":
			return "", &TaskError{Code: e.Error.Code, Message: e.Error.Message}
		}
		if committed != "" {
			if text, ok := finals[committed]; ok {
				return text, nil
			}
		}
		if len(partials) > 16 || len(finals) > 16 {
			return "", ErrProtocol
		}
	}
}

// sendOmni streams base64 PCM in JSON and submits one turn. Short/empty
// recordings are padded to 100 ms so the service never sees an empty commit.
func (s *Session) sendOmni(ws *conn, submitted *atomic.Bool) {
	sent := 0
	appendAudio := func(b []byte) error {
		return ws.WriteText(omniEvent("input_audio_buffer.append", map[string]any{
			"audio": base64.StdEncoding.EncodeToString(b),
		}))
	}
	for {
		select {
		case <-s.ctx.Done():
			return
		case b := <-s.out:
			if len(b) != 0 {
				if err := appendAudio(b); err != nil {
					s.fail(err)
					return
				}
				sent += len(b)
				continue
			}
			if sent < chunk {
				if err := appendAudio(make([]byte, chunk-sent)); err != nil {
					s.fail(err)
					return
				}
			}
			// Bound a missing final transcript independently of the UI owner.
			finalTimer := time.AfterFunc(15*time.Second, func() { s.fail(context.DeadlineExceeded) })
			go func() { <-s.ctx.Done(); finalTimer.Stop() }()
			submitted.Store(true)
			if err := ws.WriteText(omniEvent("input_audio_buffer.commit", nil)); err != nil {
				s.fail(err)
			}
			return
		}
	}
}
