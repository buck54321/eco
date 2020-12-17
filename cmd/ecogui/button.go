package main

import (
	"image/color"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/driver/desktop"
	"fyne.io/fyne/widget"
	"github.com/buck54321/eco/ui"
)

type bttnOpts struct {
	bgColor    color.Color
	hoverColor color.Color
	paddingX   int
	paddingY   int
	fontSize   int
}

type ecoBttn struct {
	widget.BaseWidget
	size      fyne.Size
	outerRect []fyne.CanvasObject
	innerRect []fyne.CanvasObject
	hoverRect []fyne.CanvasObject
	txt       *canvas.Text
	hovered   bool
	click     func()
}

func newEcoBttn(opts *bttnOpts, text string, click func(*fyne.PointEvent)) *ui.Element {
	if opts == nil {
		opts = &bttnOpts{}
	}
	nonzero := func(this, that int) int {
		if this == 0 {
			return that
		}
		return this
	}
	clr := opts.bgColor
	if clr == nil {
		clr = ui.DefaultButtonColor
	}
	hoverClr := opts.hoverColor
	if hoverClr == nil {
		hoverClr = ui.DefaultButtonHoverColor
	}
	fontSize := nonzero(opts.fontSize, 14)
	px := nonzero(opts.paddingX, 20)
	py := nonzero(opts.paddingY, 10)

	// Prepare text.
	txt := canvas.NewText(text, ui.TextColor)
	txt.TextSize = fontSize
	txt.TextStyle.Bold = true
	txt.Resize(txt.MinSize())
	// txtBox := txt.MinSize()
	// txt.Move(fyne.NewPos(paddingX+borderWidth, paddingY+borderWidth))

	var bttn *ui.Element
	bttn = ui.NewElement(&ui.Style{
		BgColor:      clr,
		Padding:      ui.FourSpec{py, px, py, px},
		BorderWidth:  1,
		BorderRadius: 2,
		Cursor:       desktop.PointerCursor,
		Display:      ui.DisplayInline,
		Listeners: ui.EventListeners{
			Click: click,
			MouseIn: func(*desktop.MouseEvent) {
				bttn.SetBackgroundColor(hoverClr)
			},
			MouseOut: func() {
				bttn.SetBackgroundColor(clr)
			},
		},
	},
		txt,
	)

	return bttn
}
