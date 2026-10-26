package bridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLinuxHelperGetsGracefulTermination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "helper")
	body := `#!/bin/sh
trap 'printf '\''%s\n'\'' '\''{"type":"model","message":"cleaned up"}'\''; exit 0' TERM
printf '%s\n' '{"type":"ready"}'
while :; do sleep 0.01; done
`
	if err := os.WriteFile(path, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	events := make(chan Event, 8)
	p, err := Start(context.Background(), path, func(e Event) { events <- e })
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	select {
	case e := <-events:
		if e.Type != "ready" {
			t.Fatal(e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("helper not ready")
	}
	p.Close()
	deadline := time.After(4 * time.Second)
	for {
		select {
		case e := <-events:
			if e.Message == "cleaned up" {
				return
			}
			if e.Type == "disconnected" {
				t.Fatal("helper was killed before cleanup")
			}
		case <-deadline:
			t.Fatal("cleanup timeout")
		}
	}
}
