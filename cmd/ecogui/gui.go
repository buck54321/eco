package main

import (
	"fmt"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/container"
	"fyne.io/fyne/widget"
)

type GUI struct {
	app    fyne.App
	window fyne.Window
	intro  *fyne.Container
}

func NewGUI() *GUI {
	a := app.New()
	fyne.CurrentApp().Settings().SetTheme(newDefaultTheme())
	w := a.NewWindow("Hello There")
	w.Resize(fyne.NewSize(1024, 768))

	intro := widget.NewLabel("")
	intro.Wrapping = fyne.TextWrapWord
	intro.BaseWidget.Resize(fyne.NewSize(450, 0))
	intro.Text = introductionText

	entry := &betterEntry{Entry: &widget.Entry{}, w: 450}
	entry.PlaceHolder = "set your password"
	entry.Password = true
	entry.ExtendBaseWidget(entry)

	bttn1 := newEcoBttn(&bttnOpts{
		bgColor:    buttonColor2,
		hoverColor: buttonHoverColor2,
	}, "Full Sync", func() {
		fmt.Println("Ooh. That felt nice.")
	})

	bttn2 := newEcoBttn(nil, "Lite Mode (SPV)", func() {
		fmt.Println("That felt so so.")
	})

	// bttn.Importance = widget.HighImportance
	logo := newSizedImage(logoRsrc, 0, 40)

	introView := container.NewCenter(fyne.NewContainerWithLayout(
		&ecoColumn{
			spacing:  30,
			yPadding: 20,
		},
		logo,
		newLabelWithWidth(intro, 450),
		entry,
		newFlexRow(&flexOpts{
			w:             450,
			justification: justifyAround,
		}, bttn1, bttn2),
	))

	w.SetContent(container.NewVBox(introView))

	return &GUI{
		app:    a,
		window: w,
		intro:  introView,
	}
}

func (gui *GUI) Run() {
	gui.window.ShowAndRun()
}
