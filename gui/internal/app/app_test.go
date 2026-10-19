package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vfasky/my-ai-audio/gui/internal/firmware"
	"github.com/vfasky/my-ai-audio/gui/internal/permission"
	"github.com/vfasky/my-ai-audio/gui/internal/platform/bridge"
	appui "github.com/vfasky/my-ai-audio/gui/internal/ui"
)

func installReady() firmware.State {
	s := firmware.Initial()
	s.Apply(firmware.Event{Type: "ready"})
	s.Scan()
	s.Apply(firmware.Event{Type: "ports", Devices: []firmware.Device{{ID: "/dev/cu.usbmodem-secret", Name: "serial-secret"}}})
	s.Select("/dev/cu.usbmodem-secret")
	return s
}
func TestInstallRefusesActiveSpeech(t *testing.T) {
	a := &application{installState: installReady()}
	a.coordinator.Active = true
	a.config.Peripheral = "existing-ble"
	a.installFirmware()
	if a.installState.Phase != "ready" || !a.coordinator.Active || a.config.Peripheral != "existing-ble" {
		t.Fatal("active speech changed")
	}
}
func TestInstallDisconnectsAndDoesNotPersistUSB(t *testing.T) {
	a := &application{installState: installReady(), directory: t.TempDir()}
	a.config.Peripheral = "old-ble"
	a.coordinator.Ready = true
	a.subscribed = true
	a.installFirmware()
	if a.config.Peripheral != "" || a.coordinator.Ready || a.subscribed {
		t.Fatal("BLE selection retained")
	}
	if a.installState.Phase != "unavailable" {
		t.Fatal("missing helper not isolated")
	}
	b, err := os.ReadFile(filepath.Join(a.directory, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "secret") || strings.Contains(string(b), "old-ble") {
		t.Fatal("device identifier persisted")
	}
}
func TestInstallBlocksBLEEvents(t *testing.T) {
	a := &application{installState: installReady()}
	if !a.installState.Install(false) {
		t.Fatal("start")
	}
	a.onBluetooth(bridge.Event{Type: "subscribed"})
	a.onBluetooth(bridge.Event{Type: "connected"})
	if a.subscribed || a.coordinator.Ready || a.view.Snapshot.Connection != "" {
		t.Fatal("BLE events accepted during install")
	}
	// App refresh snapshots expose the same gate as the reducer.
	a.refresh()
	if !a.view.Snapshot.Install.Busy() || a.view.Snapshot.Install.CanInstall(false) {
		t.Fatal("incorrect UI gate")
	}
}

func perm(name, state string) bridge.Event {
	return bridge.Event{Type: "permission", Permission: name, State: state}
}
func TestFirstCompletePermissionReportOpensOnboarding(t *testing.T) {
	a := &application{permissions: permission.Set{}, installState: firmware.Initial()}
	a.onBluetooth(perm("bluetooth", "granted"))
	a.onSpeech(perm("speech", "undetermined"))
	if a.view.Page != 0 || a.onboarded {
		t.Fatal("decided before every required grant reported")
	}
	a.onSpeech(perm("accessibility", "granted"))
	if a.view.Page != appui.PermissionsPage || !a.onboarded {
		t.Fatalf("page %d", a.view.Page)
	}
	a.view.Page = 0
	a.onSpeech(perm("speech", "denied"))
	if a.view.Page != 0 {
		t.Fatal("onboarding must only open once")
	}
}
func TestSpeechWithoutGrantEndsSessionWithReason(t *testing.T) {
	a := &application{permissions: permission.Set{}, installState: firmware.Initial(), onboarded: true}
	a.coordinator.Active = true
	a.onSpeech(perm("speech", "undetermined"))
	if a.coordinator.Active || !strings.Contains(a.coordinator.Status, "语音识别未授权") {
		t.Fatalf("status %q", a.coordinator.Status)
	}
}
