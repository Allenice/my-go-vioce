package linuxhost

import (
	"strings"
	"testing"
)

func TestCommandsRejectAmbiguousOrUnboundedInput(t *testing.T) {
	for _, line := range []string{`null`, `{"type":"begin","type":"insert"}`, `{"type":"begin","sessionId":-1}`, `{"type":"insert","text":null}`, `{"type":"scan","unknown":true}`, `{"type":"scan"} {}`, strings.Repeat(" ", 16385)} {
		if _, err := parseCommand([]byte(line)); err == nil {
			t.Fatalf("accepted %q", line)
		}
	}
	for _, line := range []string{`{"type":"scan"}`, `{"type":"set-preferred-device","deviceId":null}`, `{"type":"begin","sessionId":1}`, `{"type":"insert","sessionId":1,"text":"hello"}`} {
		if _, err := parseCommand([]byte(line)); err != nil {
			t.Fatalf("rejected %s: %v", line, err)
		}
	}
}
