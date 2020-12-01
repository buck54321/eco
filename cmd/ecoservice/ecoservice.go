package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/buck54321/eco"
)

func main() {
	var install bool
	flag.BoolVar(&install, "install", false, "Install the Eco system service.")
	flag.Parse()

	if install {
		installService()
		return
	}

	eco.InitLogging()
	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Interrupt)
	go func() {
		<-killChan
		defer fmt.Print("\r") // Delete the little '^C' printed in some (all?) linux terminals
		cancel()
	}()
	eco.Run(ctx)
}
