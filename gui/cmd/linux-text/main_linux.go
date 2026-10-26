package main

import (
	"context"
	"github.com/vfasky/my-ai-audio/gui/internal/platform/linuxhost"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	if linuxhost.RunText(ctx, os.Stdin, os.Stdout) != nil {
		os.Exit(1)
	}
}
