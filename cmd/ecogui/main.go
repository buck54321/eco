package main

import (
	"context"

	"github.com/buck54321/eco"
	"github.com/decred/slog"
)

var (
	ctx, cancel = context.WithCancel(context.Background())
	log         slog.Logger
)

func main() {
	// killChan := make(chan os.Signal)
	// signal.Notify(killChan, os.Interrupt)
	// go func() {
	// 	<-killChan
	// 	cancel()
	// }()
	log = eco.InitLogging("ecogui")
	gui := NewGUI(ctx)
	gui.Run()
}
