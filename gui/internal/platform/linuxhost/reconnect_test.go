package linuxhost

import (
	"bytes"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestReconnectIgnoresQueuedPreviousConnectionSignals(t *testing.T) {
	var output bytes.Buffer
	b := bluetooth{selected: "/org/bluez/hci0/dev_AA_BB_CC_DD_EE_FF", control: "/org/bluez/hci0/dev_AA_BB_CC_DD_EE_FF/service1/char1", minSignal: 50, out: emitter{out: &output}}
	b.signal(&dbus.Signal{Sequence: 49, Path: b.selected, Body: []any{deviceIF, map[string]dbus.Variant{"Connected": dbus.MakeVariant(false)}, []string{}}})
	b.signal(&dbus.Signal{Sequence: 50, Path: b.control, Body: []any{charIF, map[string]dbus.Variant{"Value": dbus.MakeVariant([]byte(`{"v":1,"type":"ptt_down","sessionId":1,"seq":0}`))}, []string{}}})
	if b.selected == "" || output.Len() != 0 {
		t.Fatal("old session affected new connection")
	}
	b.signal(&dbus.Signal{Sequence: 51, Path: b.control, Body: []any{charIF, map[string]dbus.Variant{"Value": dbus.MakeVariant([]byte(`{"v":1,"type":"ptt_down","sessionId":2,"seq":0}`))}, []string{}}})
	if output.Len() == 0 {
		t.Fatal("current connection ignored")
	}
}

func TestSelectingConnectedDeviceDoesNotSubscribeAgain(t *testing.T) {
	b := bluetooth{selected: "/org/bluez/hci0/dev_AA_BB_CC_DD_EE_FF"}
	b.preferred = b.selected
	// No bus is needed: a second click must not re-acquire an active socket.
	b.connectPreferred()
}
