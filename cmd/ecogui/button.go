package main

import (
	"image/color"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/driver/desktop"
	"fyne.io/fyne/widget"
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

func newEcoBttn(opts *bttnOpts, text string, click func(*fyne.PointEvent)) *Element {
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
		clr = defaultButtonColor
	}
	hoverClr := opts.hoverColor
	if hoverClr == nil {
		hoverClr = defaultButtonHoverColor
	}
	fontSize := nonzero(opts.fontSize, 14)
	px := nonzero(opts.paddingX, 20)
	py := nonzero(opts.paddingY, 10)

	// Prepare text.
	txt := canvas.NewText(text, textColor)
	txt.TextSize = fontSize
	txt.TextStyle.Bold = true
	txt.Resize(txt.MinSize())
	// txtBox := txt.MinSize()
	// txt.Move(fyne.NewPos(paddingX+borderWidth, paddingY+borderWidth))

	var bttn *Element
	bttn = newElement(&elementStyle{
		bgColor:      clr,
		padding:      borderSpecs{py, px, py, px},
		borderWidth:  1,
		borderRadius: 2,
		cursor:       desktop.PointerCursor,
		display:      displayInline,
		listeners: eventListeners{
			click: click,
			mouseIn: func(*desktop.MouseEvent) {
				bttn.setBackgroundColor(hoverClr)
			},
			mouseOut: func() {
				bttn.setBackgroundColor(clr)
			},
		},
	},
		txt,
	)

	return bttn
}
