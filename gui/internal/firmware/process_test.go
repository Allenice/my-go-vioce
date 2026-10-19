package firmware

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func script(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "helper")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}
func waitDone(t *testing.T, p *Process) {
	t.Helper()
	select {
	case <-p.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("process did not exit")
	}
}
func TestProcessRoundTripAndShutdown(t *testing.T) {
	path := script(t, `printf '%s\n' '{"type":"ready"}'
while IFS= read -r line; do
 case "$line" in
  *scan*) printf '%s\n' '{"type":"ports","devices":[]}' ;;
  *shutdown*) printf '%s\n' '{"type":"complete"}'; exit 0 ;;
 esac
done
`)
	p, err := Start(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	select {
	case e := <-p.Events():
		if e.Type != "ready" {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("ready timeout")
	}
	if err := p.Send(Request{Type: "scan"}); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-p.Events():
		if e.Type != "ports" {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("ports timeout")
	}
	p.Close()
	waitDone(t, p)
	select {
	case e := <-p.Events():
		if e.Type != "complete" {
			t.Fatal(e)
		}
	default:
		t.Fatal("shutdown request was not received")
	}
	if err := p.Send(Request{Type: "scan"}); err == nil {
		t.Fatal("send after close")
	}
}
func TestProcessInvalidExitAndOverflow(t *testing.T) {
	for _, body := range []string{"exit 1\n", `printf '%s\n' '{"type":"ready","unknown":1}'; sleep 10`, fmt.Sprintf("printf '%%s\\n' '%s'; sleep 10", strings.Repeat("x", MaxLine+1)), `i=0; while [ "$i" -lt 100 ]; do printf '%s\n' '{"type":"ready"}'; i=$((i+1)); done; sleep 10`} {
		t.Run(fmt.Sprint(len(body)), func(t *testing.T) {
			p, err := Start(context.Background(), script(t, body))
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			waitDone(t, p)
		})
	}
}
func TestConcurrentSendCloseAndBackpressure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p, err := Start(ctx, script(t, "sleep 10\n"))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 200; n++ {
				_ = p.Send(Request{Type: "scan"})
			}
		}()
	}
	wg.Add(1)
	go func() { defer wg.Done(); p.Close() }()
	wg.Wait()
	waitDone(t, p)
	// A full queue must fail immediately and cancel the process.
	_, stop := context.WithCancel(context.Background())
	q := &Process{send: make(chan []byte, 1), done: make(chan struct{}), cancel: stop}
	if q.Send(Request{Type: "scan"}) != nil {
		t.Fatal("first send")
	}
	if q.Send(Request{Type: "scan"}) == nil {
		t.Fatal("backpressure accepted")
	}
}
