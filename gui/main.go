package main

import (
	_ "embed"

	"github.com/egoist/mygo"
	"github.com/vfasky/my-ai-audio/gui/internal/app"
)

// The packaged app icon, also shown at the top of the sidebar.
//
//go:embed resources/icon.png
var icon []byte

// The menu-bar template icon, from gui/scripts/generate-tray-icon.mjs.
//
//go:embed resources/tray.png
var tray []byte

// version is the packaged release version, set by scripts/build-linux.sh from
// mygo.json so the Linux host reports the same version as the macOS app.
var version string

func main() {
	if version != "" {
		mygo.App.SetVersion(version)
	}
	app.Run(icon, tray)
}
