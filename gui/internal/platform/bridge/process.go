// Package bridge owns bounded native child processes; no audio is written to disk.
package bridge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/vfasky/my-ai-audio/gui/internal/permission"
	"github.com/vfasky/my-ai-audio/gui/internal/protocol"
	"io"
	"os/exec"
	"sync"
	"time"
)

const MaxLine = 16 * 1024

type Event struct {
	Type              string `json:"type"`
	SessionID         uint32 `json:"sessionId,omitempty"`
	Sequence          uint32 `json:"sequence,omitempty"`
	DeviceID          string `json:"deviceId,omitempty"`
	DeviceName        string `json:"deviceName,omitempty"`
	PreferredDeviceID string `json:"preferredDeviceId,omitempty"`
	PayloadBase64     string `json:"payloadBase64,omitempty"`
	Text              string `json:"text,omitempty"`
	Message           string `json:"message,omitempty"`
	Level             string `json:"level,omitempty"`
	Permission        string `json:"permission,omitempty"`
	State             string `json:"state,omitempty"`
	Payload           []byte `json:"-"`
}

func Parse(line []byte) (Event, error) {
	var e Event
	m, err := protocol.Object(line, MaxLine)
	if err != nil {
		return e, err
	}
	d := json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	if d.Decode(&e) != nil {
		return e, protocol.ErrInvalid
	}
	if len(e.DeviceID) > 256 || len(e.DeviceName) > 256 || len(e.PreferredDeviceID) > 256 || len(e.Message) > 128 || len(e.Text) > 12000 {
		return e, protocol.ErrInvalid
	}
	if e.Type != "permission" && (e.Permission != "" || e.State != "") {
		return e, protocol.ErrInvalid
	}
	switch e.Type {
	case "control", "audio":
		limit := 182
		if e.Type == "audio" {
			limit = 166
			if _, ok := m["sessionId"]; !ok {
				return e, protocol.ErrInvalid
			}
			if _, ok := m["sequence"]; !ok || e.Sequence >= protocol.MaxFrames {
				return e, protocol.ErrInvalid
			}
		}
		if len(e.PayloadBase64) > base64.StdEncoding.EncodedLen(limit) || e.PayloadBase64 == "" {
			return e, protocol.ErrInvalid
		}
		b, err := base64.StdEncoding.Strict().DecodeString(e.PayloadBase64)
		if err != nil || len(b) > limit || base64.StdEncoding.EncodeToString(b) != e.PayloadBase64 || e.Type == "audio" && len(b) != 166 {
			return e, protocol.ErrInvalid
		}
		e.Payload = b
	case "permission":
		if _, _, ok := permission.Parse(e.Permission, e.State); !ok || len(m) != 3 {
			return e, protocol.ErrInvalid
		}
	case "connected", "subscribed", "disconnected", "discovered", "error", "diagnostic", "ready", "partial", "final", "cancelled", "refined", "inserted", "model":
	default:
		return e, protocol.ErrInvalid
	}
	return e, nil
}

type Process struct {
	cmd    *exec.Cmd
	in     io.WriteCloser
	send   chan []byte
	done   chan struct{}
	cancel context.CancelFunc
	once   sync.Once
}

func Start(ctx context.Context, path string, receive func(Event)) (*Process, error) {
	ctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(ctx, path)
	cmd.WaitDelay = time.Second
	in, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	// Native diagnostics are intentionally not forwarded into logs.
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		cancel()
		return nil, err
	}
	p := &Process{cmd: cmd, in: in, send: make(chan []byte, 128), done: make(chan struct{}), cancel: cancel}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case b := <-p.send:
				if _, e := in.Write(b); e != nil {
					cancel()
					return
				}
			}
		}
	}()
	go func() {
		s := bufio.NewScanner(out)
		s.Buffer(make([]byte, 4096), MaxLine+1)
		for s.Scan() {
			e, err := Parse(s.Bytes())
			if err != nil {
				cancel()
				break
			}
			receive(e)
		}
		_ = cmd.Wait()
		cancel()
		close(p.done)
		receive(Event{Type: "disconnected"})
	}()
	return p, nil
}
func (p *Process) Send(v any) error {
	b, e := json.Marshal(v)
	if e != nil || len(b) > MaxLine {
		return protocol.ErrInvalid
	}
	b = append(b, '\n')
	select {
	case <-p.done:
		return errors.New("native process stopped")
	default:
	}
	select {
	case p.send <- b:
		return nil
	default:
		return errors.New("native pipe backpressure")
	}
}
func (p *Process) Close() { p.once.Do(func() { _ = p.in.Close(); p.cancel() }) }
