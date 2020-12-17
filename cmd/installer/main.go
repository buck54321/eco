package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/canvas"
	"github.com/buck54321/eco"
	"github.com/buck54321/eco/ui"
)

var (
	osUser, _ = user.Current()
)

func main() {
	a := app.New()
	a.Settings().SetTheme(ui.NewDefaultTheme())
	w := a.NewWindow("Decred Eco")
	// w.SetIcon(windowLogo)
	w.Resize(fyne.NewSize(400, 150))

	progressLbl := ui.NewEcoLabel("beginning installation", &ui.TextStyle{
		FontSize: 16,
	})
	progressLbl.Refresh()

	mainWin := ui.NewElement(&ui.Style{
		Justi:            ui.JustifyCenter,
		Align:            ui.AlignCenter,
		Spacing:          20,
		ExpandVertically: true,
	},
		progressLbl,
	)
	mainWin.Name = "mainWin"

	setProgress := func(s string, a ...interface{}) {
		progressLbl.SetText(s, a...)
		// progressLbl.Refresh()
		mainWin.Refresh()
		canvas.Refresh(mainWin)
	}

	go func() {
		time.Sleep(time.Second)
		_, err := eco.ExtractTarGz("unpacked", bytes.NewReader(archive))
		if err != nil {
			setProgress("error unpacking: %v", err)
			return
		}
		defer os.RemoveAll("unpacked")

		logoB, err := ioutil.ReadFile(filepath.Join("unpacked", "static", "eco-logo.png"))
		if err != nil {
			setProgress("logo file read error: %v", err)
			return
		}

		img := ui.NewSizedImage(fyne.NewStaticResource("logo.png", logoB), 0, 35)
		mainWin.InsertChild(img, 0)

		setProgress("Moving files")
		err = moveFiles()
		if err != nil {
			setProgress("Error moving files: %v", err)
			return
		}

		setProgress("Initializing system service")
		err = initService()
		if err != nil {
			setProgress("Error initializing service: %v", err)
			return
		}
	}()
	w.SetContent(mainWin)
	w.ShowAndRun()
}

func moveFile(sourcePath, destDir string) error {
	inputFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("Couldn't open source file: %s", err)
	}
	defer inputFile.Close()
	inputInfo, err := inputFile.Stat()
	if err != nil {
		return fmt.Errorf("%q Stat error: %v", sourcePath, err)
	}
	if _, err := os.Stat(destDir); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("Stat error: %v", err)
		}
		err := os.MkdirAll(destDir, 0755)
		if err != nil {
			return fmt.Errorf("MkdirAll error for %q: %v", destDir, err)
		}
	}

	destPath := filepath.Join(destDir, filepath.Base(sourcePath))
	outputFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, inputInfo.Mode())
	if err != nil {
		return fmt.Errorf("Couldn't open dest file: %s", err)
	}
	defer outputFile.Close()
	_, err = io.Copy(outputFile, inputFile)
	if err != nil {
		return fmt.Errorf("Writing to output file failed: %s", err)
	}
	// The copy was successful, so now delete the original file
	err = os.Remove(sourcePath)
	if err != nil {
		return fmt.Errorf("Failed removing original file: %s", err)
	}
	return nil
}
