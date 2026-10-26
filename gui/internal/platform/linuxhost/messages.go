// Package linuxhost implements the Linux native helpers using the same bounded
// NDJSON contract as the macOS helpers. Nothing records audio or transcripts.
package linuxhost

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"
	"unicode/utf8"

	"github.com/vfasky/my-ai-audio/gui/internal/platform/bridge"
	"github.com/vfasky/my-ai-audio/gui/internal/protocol"
)

type Command struct {
	Type          string   `json:"type"`
	DeviceID      string   `json:"deviceId"`
	SessionID     uint32   `json:"sessionId"`
	Text          string   `json:"text"`
	State         string   `json:"state"`
	Terms         []string `json:"terms"`
	Sequence      uint32   `json:"sequence"`
	PayloadBase64 string   `json:"payloadBase64"`
}

type emitter struct {
	mu  sync.Mutex
	out io.Writer
}

func (e *emitter) send(v bridge.Event) {
	b, err := json.Marshal(v)
	if err != nil || len(b) > bridge.MaxLine {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	_, _ = e.out.Write(append(b, '\n'))
}
func (e *emitter) permission(name, state string) {
	e.send(bridge.Event{Type: "permission", Permission: name, State: state})
}

func parseCommand(line []byte) (Command, error) {
	var c Command
	if len(line) > bridge.MaxLine || !utf8.Valid(line) {
		return c, protocol.ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(line))
	first, err := d.Token()
	if err != nil || first != json.Delim('{') {
		return c, protocol.ErrInvalid
	}
	m := map[string]json.RawMessage{}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return c, protocol.ErrInvalid
		}
		key, ok := token.(string)
		if !ok {
			return c, protocol.ErrInvalid
		}
		if _, exists := m[key]; exists {
			return c, protocol.ErrInvalid
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return c, protocol.ErrInvalid
		}
		if bytes.Equal(value, []byte("null")) && key != "deviceId" {
			return c, protocol.ErrInvalid
		}
		m[key] = value
	}
	if _, err = d.Token(); err != nil {
		return c, protocol.ErrInvalid
	}
	if _, err = d.Token(); err != io.EOF {
		return c, protocol.ErrInvalid
	}
	clean, _ := json.Marshal(m)
	d = json.NewDecoder(bytes.NewReader(clean))
	d.DisallowUnknownFields()
	if d.Decode(&c) != nil || c.Type == "" {
		return c, protocol.ErrInvalid
	}
	return c, nil
}

// EOF, oversized input, or queue backpressure cancels the whole helper. The
// parent must never keep a half-dead native session after a broken pipe.
func commands(ctx context.Context, cancel context.CancelFunc, in io.Reader) <-chan Command {
	ch := make(chan Command, 128)
	go func() {
		defer cancel()
		s := bufio.NewScanner(in)
		s.Buffer(make([]byte, 4096), bridge.MaxLine+1)
		for s.Scan() {
			c, err := parseCommand(s.Bytes())
			if err != nil {
				return
			}
			select {
			case ch <- c:
			case <-ctx.Done():
				return
			default:
				return
			}
		}
	}()
	return ch
}
