package firmware

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	valid := []string{`{"type":"ready"}`, `{"type":"complete"}`, `{"type":"ports","devices":[]}`, `{"type":"ports","devices":[{"id":"/dev/cu.usbmodem1","name":"Go Voice"}]}`, `{"type":"progress","stage":"writing","percent":5,"message":"Writing"}`, `{"type":"error","code":"failed","message":"Device unavailable","recoverable":true}`}
	for _, line := range valid {
		if _, err := Parse([]byte(line)); err != nil {
			t.Errorf("valid %s: %v", line, err)
		}
	}
	invalid := []string{`null`, `{"type":"ready","extra":1}`, `{"type":"ready","type":"ready"}`, `{"type":"ready"} {}`, `{"type":"ready","percent":0}`, `{"type":"ports","devices":null}`, `{"type":"ports","devices":[{"id":"/dev/cu.a","id":"/dev/cu.b","name":"a"}]}`, `{"type":"ports","devices":[{"id":"/dev/cu.a","name":"a","serial":"secret"}]}`, `{"type":"ports","devices":[{"id":"/dev/cu.a","name":"a"},{"id":"/dev/cu.a","name":"a"}]}`, `{"type":"ports","devices":[{"id":"/dev/cu.a/../../etc","name":"a"}]}`, `{"type":"progress","stage":"writing","percent":101,"message":"x"}`, `{"type":"progress","stage":"writing","percent":-1,"message":"x"}`, `{"type":"progress","stage":"writing","percent":1.5,"message":"x"}`, `{"type":"progress","stage":"writing","percent":null,"message":"x"}`, `{"type":"progress","stage":"writing","message":"x"}`, `{"type":"progress","stage":"other","percent":0,"message":"x"}`, `{"type":"error","code":"x","message":"/Users/private","recoverable":true}`, `{"type":"error","code":"x","message":"a\nb","recoverable":true}`, `{"type":"error","code":"x","message":"a\u202eb","recoverable":true}`, `{"type":"error","code":"x","message":"x"}`}
	invalid = append(invalid, `{"type":"ports","devices":[`+strings.TrimSuffix(strings.Repeat(`{"id":"/dev/cu.a","name":"a"},`, 33), ",")+`]}`, strings.Repeat(" ", MaxLine)+`{"type":"ready"}`)
	for _, line := range invalid {
		if _, err := Parse([]byte(line)); err == nil {
			t.Errorf("accepted %s", line)
		}
	}
}
func readyState() State {
	s := Initial()
	s.Apply(Event{Type: "ready"})
	s.Scan()
	s.Apply(Event{Type: "ports", Devices: []Device{{ID: "/dev/cu.a", Name: "USB"}}})
	s.Select("/dev/cu.a")
	return s
}
func TestReducer(t *testing.T) {
	s := Initial()
	if s.CanInstall(false) || s.Scan() {
		t.Fatal("unavailable enabled")
	}
	s = readyState()
	s.Select("/dev/cu.unknown")
	if s.Selected != "/dev/cu.a" {
		t.Fatal("unknown selected")
	}
	if s.Install(true) || !s.Install(false) || s.Install(false) || s.Scan() {
		t.Fatal("install gates")
	}
	s.Apply(Event{Type: "ports"})
	s.Apply(Event{Type: "ready"})
	s.Apply(Event{Type: "complete"})
	s.Apply(Event{Type: "progress", Stage: "verifying", Percent: 100})
	if s.Phase != "preflight" {
		t.Fatal("out-of-order event accepted")
	}
	for _, stage := range []string{"preflight", "writing", "verifying", "restarting"} {
		s.Apply(Event{Type: "progress", Stage: stage, Percent: 50})
		s.Apply(Event{Type: "progress", Stage: stage, Percent: 10})
		if s.Phase != stage || s.Percent != 50 {
			t.Fatalf("regression %+v", s)
		}
	}
	if !s.Apply(Event{Type: "complete"}) || s.Apply(Event{Type: "complete"}) || s.Selected != "" || s.Busy() {
		t.Fatal("completion")
	}
	s.Apply(Event{Type: "progress", Stage: "writing", Percent: 1})
	if s.Phase != "complete" {
		t.Fatal("late progress")
	}
	if !s.Scan() {
		t.Fatal("cannot rescan")
	}
	s.Apply(Event{Type: "error", Code: "INVALID_LAYOUT", Message: "refused", Recoverable: true})
	if s.ErrorCode != "INVALID_LAYOUT" || s.FailedStage != "scanning" || !strings.Contains(s.Message, "写入前停止") {
		t.Fatal("error diagnostics", s)
	}
	if !s.Scan() {
		t.Fatal("recoverable")
	}
	s.Apply(Event{Type: "error"})
	if s.Scan() {
		t.Fatal("fatal enabled")
	}
	s.Unavailable()
	if s.HelperReady || s.Selected != "" {
		t.Fatal("exit")
	}
}
func TestErrorLabels(t *testing.T) {
	for _, code := range []string{"HASH_MISMATCH", "PORT_NOT_ALLOWED", "CONNECT_FAILED", "UNSAFE_SECURITY_STATE", "PREFLIGHT_READ_UNAVAILABLE", "FLASH_FAILED", "OUTPUT_FAILED", "UNKNOWN"} {
		if label := ErrorLabel(code); label == "" || strings.ContainsAny(label, "/\\") {
			t.Fatalf("unsafe label for %s: %q", code, label)
		}
	}
}
func TestRequestsAndPaths(t *testing.T) {
	p := BundlePaths("/Applications/Go Voice.app/Contents/MacOS/GoVoice")
	if p.Helper != "/Applications/Go Voice.app/Contents/Resources/native/FirmwareHelper.app/Contents/MacOS/FirmwareHelper" {
		t.Fatal(p)
	}
	for _, r := range []Request{{Type: "scan"}, {Type: "shutdown"}, {Type: "install", Port: "/dev/cu.a", FirmwarePath: p.Firmware, ManifestPath: p.Manifest}} {
		if _, err := encode(r); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []Request{{Type: "unknown"}, {Type: "scan", Port: "/dev/cu.a"}, {Type: "install", Port: "/tmp/file", FirmwarePath: p.Firmware, ManifestPath: p.Manifest}, {Type: "install", Port: "/dev/cu.a", FirmwarePath: "../a", ManifestPath: p.Manifest}} {
		if _, err := encode(r); err == nil {
			t.Fatal("accepted", r)
		}
	}
}
