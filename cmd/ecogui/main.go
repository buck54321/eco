package main

import (
	"context"
	"os"

	"github.com/buck54321/eco"
	"github.com/decred/slog"
)

var (
	ctx, cancel = context.WithCancel(context.Background())
	log         = slog.NewBackend(os.Stdout).Logger("GUI")
)

func main() {
	// killChan := make(chan os.Signal)
	// signal.Notify(killChan, os.Interrupt)
	// go func() {
	// 	<-killChan
	// 	cancel()
	// }()
	eco.UseLogger(slog.NewBackend(os.Stdout).Logger("ECO"))
	gui := NewGUI(ctx)
	gui.Run()
}
