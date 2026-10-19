package firmware

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type Request struct {
	Type         string `json:"type"`
	Port         string `json:"port,omitempty"`
	FirmwarePath string `json:"firmwarePath,omitempty"`
	ManifestPath string `json:"manifestPath,omitempty"`
}
type Paths struct{ Helper, Firmware, Manifest string }

func BundlePaths(executable string) Paths {
	resources := filepath.Clean(filepath.Join(filepath.Dir(executable), "..", "Resources"))
	return Paths{filepath.Join(resources, "native", "FirmwareHelper.app", "Contents", "MacOS", "FirmwareHelper"), filepath.Join(resources, "firmware", "go-voice-app.bin"), filepath.Join(resources, "firmware", "manifest.json")}
}
func validPath(s string) bool {
	return filepath.IsAbs(s) && filepath.Clean(s) == s && safeText(s, 4096)
}
func encode(r Request) ([]byte, error) {
	switch r.Type {
	case "scan", "shutdown":
		if r.Port != "" || r.FirmwarePath != "" || r.ManifestPath != "" {
			return nil, errors.New("unexpected request fields")
		}
	case "install":
		if !ValidPort(r.Port) || !validPath(r.FirmwarePath) || !validPath(r.ManifestPath) {
			return nil, errors.New("invalid install paths")
		}
	default:
		return nil, errors.New("invalid request")
	}
	b, err := json.Marshal(r)
	if err != nil || len(b) > MaxLine {
		return nil, errors.New("invalid request size")
	}
	return append(b, '\n'), nil
}

// Process queues are bounded. Consumers poll Events on their own UI cadence;
// overflow terminates the helper rather than dropping terminal/progress events.
type Process struct {
	events  chan Event
	send    chan []byte
	done    chan struct{}
	cancel  context.CancelFunc
	mu      sync.Mutex
	closing bool
}

func (p *Process) Events() <-chan Event  { return p.events }
func (p *Process) Done() <-chan struct{} { return p.done }
func Start(parent context.Context, path string) (*Process, error) {
	ctx, cancel := context.WithCancel(parent)
	cmd := exec.CommandContext(ctx, path)
	cmd.WaitDelay = time.Second
	in, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		_ = in.Close()
		cancel()
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		_ = in.Close()
		_ = out.Close()
		cancel()
		return nil, err
	}
	p := &Process{events: make(chan Event, 64), send: make(chan []byte, 8), done: make(chan struct{}), cancel: cancel}
	// A descendant holding inherited pipes must not keep our reader/writer alive.
	go func() { <-ctx.Done(); _ = in.Close(); _ = out.Close() }()
	go func() {
		defer in.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case b := <-p.send:
				if _, err := in.Write(b); err != nil {
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
				break
			}
			select {
			case p.events <- e:
			default:
				cancel()
			}
			if ctx.Err() != nil {
				break
			}
		}
		cancel()
		_ = cmd.Wait()
		p.mu.Lock()
		p.closing = true
		p.mu.Unlock()
		close(p.done)
	}()
	return p, nil
}
func (p *Process) Send(r Request) error {
	b, err := encode(r)
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closing {
		return errors.New("installer stopped")
	}
	select {
	case <-p.done:
		return errors.New("installer stopped")
	default:
	}
	select {
	case p.send <- b:
		return nil
	default:
		p.closing = true
		p.cancel()
		return errors.New("installer backpressure")
	}
}

// Close gives the helper a bounded opportunity to handle shutdown without blocking UI.
func (p *Process) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closing {
		p.cancel()
		return
	}
	p.closing = true
	select {
	case p.send <- []byte("{\"type\":\"shutdown\"}\n"):
	default:
		p.cancel()
	}
	go func() {
		select {
		case <-p.done:
		case <-time.After(500 * time.Millisecond):
			p.cancel()
		}
	}()
}
