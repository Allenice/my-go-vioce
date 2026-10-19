//go:build darwin

package bridge

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestSpeechBridgePersistentPipe guards the real app contract: the parent keeps
// stdin open for the process lifetime, so a complete NDJSON line must be
// handled immediately rather than waiting for a full read buffer or EOF.
func TestSpeechBridgePersistentPipe(t *testing.T) {
	path := filepath.Join("..", "..", "..", "macos", ".build", "debug", "SpeechBridge")
	if _, err := os.Stat(path); err != nil {
		t.Skip("build the Swift package before running the native pipe test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	events := make(chan Event, 8)
	process, err := Start(ctx, path, func(event Event) {
		select {
		case events <- event:
		default:
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer process.Close()
	if err := process.Send(map[string]any{"type": "ping"}); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case <-ctx.Done():
			t.Fatal("SpeechBridge did not answer while stdin remained open")
		case event := <-events:
			if event.Type == "model" && event.Message == "SpeechBridge ready" {
				return
			}
			if event.Type == "disconnected" {
				t.Fatal("SpeechBridge exited before answering ping")
			}
		}
	}
}

// TestBluetoothHelperPersistentPipe verifies that control commands are handled
// while the parent keeps stdin open. Shutdown is intentionally permission-free
// and catches regressions where FileHandle waits for a full buffer or EOF.
func TestBluetoothHelperPersistentPipe(t *testing.T) {
	path := filepath.Join("..", "..", "..", "macos", ".build", "debug", "BluetoothHelper")
	if _, err := os.Stat(path); err != nil {
		t.Skip("build the Swift package before running the native pipe test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path)
	cmd.Env = append(os.Environ(), "GO_VOICE_BRIDGE_PIPE_TEST=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err = stdin.Write([]byte("{\"type\":\"shutdown\"}\n")); err != nil {
		t.Fatal(err)
	}
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	select {
	case err = <-wait:
		if err != nil {
			t.Fatalf("BluetoothHelper pipe-test exit: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("BluetoothHelper did not process shutdown while stdin remained open")
	}
}
