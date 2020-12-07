package main

import (
	"context"
	"os"

	"github.com/buck54321/eco"
	"github.com/decred/slog"
)

const (
	regularFont = "SourceSans3-Regular.ttf"
	boldFont    = "source-sans-pro-semibold.ttf"

	introductionText = "For the best security and the full range of Decred services, you'll want to sync the full blockchain, which will use around 5 GB of disk space. If you're only interested in basic wallet functionality, you may choose to sync in SPV mode, which will be very fast and use about 100 MB of disk space."
)

var (
	ctx, cancel = context.WithCancel(context.Background())
	log         = slog.NewBackend(os.Stdout).Logger("GUI")

	bttnColor               = stringToColor("#005")
	bgColor                 = stringToColor("#000008")
	transparent             = stringToColor("#0000")
	defaultButtonColor      = stringToColor("#003")
	defaultButtonHoverColor = stringToColor("#005")
	buttonColor2            = stringToColor("#001a08")
	buttonHoverColor2       = stringToColor("#00251a")
	textColor               = stringToColor("#c1c1c1")
	cursorColor             = stringToColor("#2970fe")
	focusColor              = cursorColor
	black                   = stringToColor("#000")
	inputColor              = stringToColor("#111")

	logoRsrc = mustLoadStaticResource("eco-logo.png")
)

func main() {
	// killChan := make(chan os.Signal)
	// signal.Notify(killChan, os.Interrupt)
	// go func() {
	// 	<-killChan
	// 	cancel()
	// }()
	eco.UseLogger(slog.NewBackend(os.Stdout).Logger("ECO"))
	gui := NewGUI()
	gui.Run(ctx)
}
