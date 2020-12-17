package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/widget"
)

type WidthLabel struct {
	*widget.Label
	w int
}

func NewLabelWithWidth(lbl *widget.Label, w int) *WidthLabel {
	return &WidthLabel{
		Label: lbl,
		w:     w,
	}
}

func (b *WidthLabel) MinSize() fyne.Size {
	sz := b.Label.MinSize()
	return fyne.NewSize(b.w, sz.Height)
}

func (b *WidthLabel) Size() fyne.Size {
	return b.MinSize()
}

func (b *WidthLabel) Resize(sz fyne.Size) {
	b.Label.Resize(fyne.NewSize(b.w, sz.Height))
}

type TextStyle struct {
	Color     color.Color
	FontSize  int
	Bold      bool
	Italic    bool
	Monospace bool
}

type EcoLabel struct {
	*Element
	txt *canvas.Text
}

func NewEcoLabel(txt string, style *TextStyle, clicks ...func(*fyne.PointEvent)) *EcoLabel {
	if style == nil {
		style = &TextStyle{}
	}
	var color color.Color
	if style.Color != nil {
		color = style.Color
	} else {
		color = TextColor
	}
	fontSize := 14
	if style.FontSize != 0 {
		fontSize = style.FontSize
	}

	fyneTxt := canvas.NewText(txt, color)
	fyneTxt.TextSize = fontSize
	fyneTxt.TextStyle = fyne.TextStyle{
		Bold:      style.Bold,
		Italic:    style.Italic,
		Monospace: style.Monospace,
	}

	var click func(*fyne.PointEvent)
	if len(clicks) > 0 {
		click = clicks[0]
	}

	lbl := &EcoLabel{
		Element: NewElement(&Style{
			Display: DisplayInline,
			Listeners: EventListeners{
				Click: click,
			},
		}, fyneTxt),
		txt: fyneTxt,
	}

	return lbl
}

func (lbl *EcoLabel) SetText(s string, a ...interface{}) {
	lbl.txt.Text = fmt.Sprintf(s, a...)
	lbl.txt.Refresh()
	// lbl.txt.Resize(lbl.txt.MinSize())
}
