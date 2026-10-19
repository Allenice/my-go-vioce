package main

import (
	_ "embed"

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

func main() { app.Run(icon, tray) }
