package ui

import (
	"github.com/vfasky/my-ai-audio/gui/internal/permission"
	"testing"
)

func TestLinuxPermissionsExplainClipboardAndRecheck(t *testing.T) {
	var rec intents
	v := testView(&rec)
	v.Page = PermissionsPage
	v.Snapshot.Permissions = permission.Set{permission.Bluetooth: permission.Denied, permission.Accessibility: permission.Granted}
	tt := newTester(v)
	if !tt.HasText("Wayland 使用 wl-copy") || tt.HasText("System Events") {
		t.Fatalf("%q", tt.Texts())
	}
	if err := tt.Click("重新检查"); err != nil || len(rec.settings) != 1 {
		t.Fatalf("recheck: %v %+v", err, rec)
	}
}

func TestLinuxDoesNotOfferUSBInstall(t *testing.T) {
	var rec intents
	v := testView(&rec)
	v.Page = 4
	tt := newTester(v)
	if !tt.HasText("Linux 预览版暂不提供 USB 固件安装") || tt.HasText("确认安装随附固件") {
		t.Fatalf("%q", tt.Texts())
	}
}
