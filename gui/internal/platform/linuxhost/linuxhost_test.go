package linuxhost

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/vfasky/my-ai-audio/gui/internal/platform/bridge"
	"github.com/vfasky/my-ai-audio/gui/internal/protocol"
)

func fixture() (objects, dbus.ObjectPath) {
	device := dbus.ObjectPath("/org/bluez/hci0/dev_AA_BB_CC_DD_EE_FF")
	service := dbus.ObjectPath(string(device) + "/service0001")
	all := objects{service: {serviceIF: {"Device": dbus.MakeVariant(device), "UUID": dbus.MakeVariant(protocol.ServiceUUID)}}}
	for i, uuid := range []string{protocol.ControlUUID, protocol.AudioUUID, protocol.InfoUUID} {
		flags := [][]string{{"write", "notify"}, {"notify"}, {"read"}}[i]
		all[dbus.ObjectPath(string(service)+"/char"+string(rune('a'+i)))] = map[string]map[string]dbus.Variant{charIF: {"Service": dbus.MakeVariant(service), "UUID": dbus.MakeVariant(uuid), "Flags": dbus.MakeVariant(flags)}}
	}
	return all, device
}
func TestCharacteristicsBelongToSelectedVoiceService(t *testing.T) {
	all, device := fixture()
	got, err := selectCharacteristics(all, device)
	if err != nil || len(got) != 3 {
		t.Fatalf("%v %v", got, err)
	}
	if _, err = selectCharacteristics(all, "/org/bluez/hci0/dev_11_22_33_44_55_66"); err == nil {
		t.Fatal("accepted another device")
	}
	control := all[got[protocol.ControlUUID]][charIF]
	control["Flags"] = dbus.MakeVariant([]string{"write-without-response", "notify"})
	if _, err = selectCharacteristics(all, device); err == nil {
		t.Fatal("accepted unacknowledged control writes")
	}
}
func TestDuplicateCharacteristicsAreRejected(t *testing.T) {
	all, device := fixture()
	chars, _ := selectCharacteristics(all, device)
	all[dbus.ObjectPath(string(chars[protocol.AudioUUID])+"duplicate")] = all[chars[protocol.AudioUUID]]
	if _, err := selectCharacteristics(all, device); err == nil {
		t.Fatal("duplicate accepted")
	}
}
func TestAudioUsesExistingAssemblerAndPreservesZeroIDs(t *testing.T) {
	var output bytes.Buffer
	b := bluetooth{out: emitter{out: &output}}
	frame := make([]byte, protocol.FrameBytes)
	binary.LittleEndian.PutUint16(frame[3:], 320)
	// Two out-of-order notifications, including a duplicate fragment.
	fragment := func(index byte, payload []byte) []byte {
		p := make([]byte, 16+len(payload))
		binary.LittleEndian.PutUint16(p, 0x5041)
		p[2] = 1
		p[12] = index
		p[13] = 2
		binary.LittleEndian.PutUint16(p[14:], uint16(len(payload)))
		copy(p[16:], payload)
		return p
	}
	for _, p := range [][]byte{fragment(1, frame[80:]), fragment(1, frame[80:]), fragment(0, frame[:80])} {
		f, err := b.assembler.Push(p)
		if err != nil {
			t.Fatal(err)
		}
		if f != nil {
			b.audio(f)
		}
	}
	e, err := bridge.Parse(bytes.TrimSpace(output.Bytes()))
	if err != nil || e.Type != "audio" || e.Sequence != 0 || e.SessionID != 0 || !bytes.Equal(e.Payload, frame) {
		t.Fatalf("event: %+v %v", e, err)
	}
}

type capture struct {
	sync.Mutex
	bytes.Buffer
}

func (c *capture) Write(b []byte) (int, error) { c.Lock(); defer c.Unlock(); return c.Buffer.Write(b) }
func (c *capture) events(t *testing.T) []bridge.Event {
	t.Helper()
	c.Lock()
	defer c.Unlock()
	var out []bridge.Event
	d := json.NewDecoder(bytes.NewReader(c.Buffer.Bytes()))
	for {
		var e bridge.Event
		if err := d.Decode(&e); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		out = append(out, e)
	}
	return out
}
func TestWaylandCopiesWithoutInvokingX11(t *testing.T) {
	output := &capture{}
	var copied string
	tx := textSession{wayland: true, out: emitter{out: output}, run: func(ctx context.Context, name string, args []string, input string, capture bool) (string, error) {
		if name != "wl-copy" {
			t.Errorf("unexpected %s", name)
		}
		copied = input
		return "", nil
	}}
	tx.begin(context.Background(), 1)
	tx.insert(1, "hello $(touch /tmp/never) `literal`")
	tx.workers.Wait()
	if copied != "hello $(touch /tmp/never) `literal`" {
		t.Fatal("text changed")
	}
	e := output.events(t)
	if len(e) != 1 || e[0].Type != "inserted" || !strings.Contains(e[0].Text, "剪贴板") {
		t.Fatalf("%+v", e)
	}
}
func TestX11FocusChangePreventsPaste(t *testing.T) {
	output := &capture{}
	reads := 0
	tx := textSession{out: emitter{out: output}, run: func(ctx context.Context, name string, args []string, input string, capture bool) (string, error) {
		if name == "xclip" {
			return "", nil
		}
		if len(args) == 1 && args[0] == "getactivewindow" {
			reads++
			if reads == 1 {
				return "12", nil
			}
			return "13", nil
		}
		t.Error("must not paste after focus changed")
		return "", errors.New("unexpected")
	}}
	tx.begin(context.Background(), 1)
	tx.insert(1, "text")
	tx.workers.Wait()
	if e := output.events(t); len(e) != 1 || e[0].Text != "已复制到剪贴板，请在目标应用中粘贴" {
		t.Fatalf("%+v", e)
	}
}
func TestCancelDuringCopyNeverPastesOrCompletes(t *testing.T) {
	output := &capture{}
	entered := make(chan struct{})
	tx := textSession{out: emitter{out: output}, run: func(ctx context.Context, name string, args []string, input string, capture bool) (string, error) {
		if name == "xclip" {
			close(entered)
			<-ctx.Done()
			return "", ctx.Err()
		}
		if len(args) == 1 && args[0] == "getactivewindow" {
			return "12", nil
		}
		t.Error("late paste")
		return "", nil
	}}
	tx.begin(context.Background(), 1)
	tx.insert(1, "text")
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("copy not started")
	}
	tx.stop(1)
	tx.workers.Wait()
	tx.insert(1, "stale")
	tx.workers.Wait()
	if len(output.events(t)) != 0 {
		t.Fatal("cancelled insertion completed")
	}
}
func TestClipboardFailureIsNotSuccess(t *testing.T) {
	output := &capture{}
	tx := textSession{wayland: true, out: emitter{out: output}, run: func(context.Context, string, []string, string, bool) (string, error) {
		return "", errors.New("unavailable")
	}}
	tx.begin(context.Background(), 2)
	tx.insert(2, "text")
	tx.workers.Wait()
	tx.stop(0)
	e := output.events(t)
	if len(e) != 1 || e[0].Type != "error" {
		t.Fatalf("%+v", e)
	}
}
