package main

import (
	"fmt"
	"image/color"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/widget"
)

type widthLabel struct {
	*widget.Label
	w int
}

func newLabelWithWidth(lbl *widget.Label, w int) *widthLabel {
	return &widthLabel{
		Label: lbl,
		w:     w,
	}
}

func (b *widthLabel) MinSize() fyne.Size {
	sz := b.Label.MinSize()
	return fyne.NewSize(b.w, sz.Height)
}

func (b *widthLabel) Size() fyne.Size {
	return b.MinSize()
}

func (b *widthLabel) Resize(sz fyne.Size) {
	b.Label.Resize(fyne.NewSize(b.w, sz.Height))
}

type textStyle struct {
	color     color.Color
	fontSize  int
	bold      bool
	italic    bool
	monospace bool
}

type ecoLabel struct {
	*Element
	txt *canvas.Text
}

func newEcoLabel(txt string, style *textStyle, clicks ...func(*fyne.PointEvent)) *ecoLabel {
	if style == nil {
		style = &textStyle{}
	}
	var color color.Color
	if style.color != nil {
		color = style.color
	} else {
		color = textColor
	}
	fontSize := 14
	if style.fontSize != 0 {
		fontSize = style.fontSize
	}

	fyneTxt := canvas.NewText(txt, color)
	fyneTxt.TextSize = fontSize
	fyneTxt.TextStyle = fyne.TextStyle{
		Bold:      style.bold,
		Italic:    style.italic,
		Monospace: style.monospace,
	}

	var click func(*fyne.PointEvent)
	if len(clicks) > 0 {
		click = clicks[0]
	}

	lbl := &ecoLabel{
		Element: newElement(&elementStyle{
			display: displayInline,
			listeners: eventListeners{
				click: click,
			},
		}, fyneTxt),
		txt: fyneTxt,
	}

	return lbl
}

func (lbl *ecoLabel) setText(s string, a ...interface{}) {
	lbl.txt.Text = fmt.Sprintf(s, a...)
	lbl.txt.Refresh()
	// lbl.txt.Resize(lbl.txt.MinSize())
}

type clickyLabel struct {
	widget.Label
	click func()
}

func newClickyLabel(txt string, click func()) *clickyLabel {
	return &clickyLabel{
		Label: *widget.NewLabel(txt),
		click: click,
	}
}

func (lbl *clickyLabel) Tapped(*fyne.PointEvent) {
	lbl.click()
}

// func (lbl *clickyLabel) Size() fyne.Size {
// 	return lbl.MinSize()
// }

// func (lbl *clickyLabel) Resize(sz fyne.Size) {
// 	lbl.Label.Resize(lbl.MinSize())
// }
