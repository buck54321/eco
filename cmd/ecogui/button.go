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

var _ fyne.Widget = (*ecoBttn)(nil)

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

	// // This top section can all be added to bttnOpts as needed.
	// borderRadius := 2
	// borderWidth := 1

	// // Prepare text.
	// txt := canvas.NewText(text, textColor)
	// txt.TextSize = fontSize
	// txt.TextStyle.Bold = true
	// txtBox := txt.MinSize()
	// txt.Move(fyne.NewPos(paddingX+borderWidth, paddingY+borderWidth))

	// // Some dims.
	// outerX := txtBox.Width + 2*paddingX + 2*borderWidth
	// outerY := txtBox.Height + 2*paddingY + 2*borderWidth
	// innerX := outerX - 2*borderWidth
	// innerY := outerY - 2*borderWidth

	// // There are nine shapes that make up a rounded rectangle. 4 corner circles,
	// // 4 side lines, and 1 center rect
	// roundedRect := func(w, h, offsetX, offsetY int, c color.Color) []fyne.CanvasObject {
	// 	r := float32(borderRadius) * 2 * float32(w) / float32(outerX)
	// 	return roundedRectangle(w, h, offsetX, offsetY, borderRadius, r, c)
	// }

	// bttn := &ecoBttn{
	// 	size:      fyne.NewSize(outerX, outerY),
	// 	outerRect: roundedRect(outerX, outerY, 0, 0, defaultBorderColor),
	// 	innerRect: roundedRect(innerX, innerY, borderWidth, borderWidth, clr),
	// 	hoverRect: roundedRect(innerX, innerY, borderWidth, borderWidth, hoverClr),
	// 	txt:       txt,
	// 	click:     click,
	// }
	// swapViz(bttn.hoverRect, nil)
	// return bttn
}

func (b *ecoBttn) CreateRenderer() fyne.WidgetRenderer {
	b.BaseWidget.ExtendBaseWidget(b)
	return &ecoBttnRenderer{bttn: b}
}

func (b *ecoBttn) MinSize() fyne.Size {
	return b.size
}

func (b *ecoBttn) Tapped(*fyne.PointEvent) {
	b.click()
}

func (b *ecoBttn) MouseIn(*desktop.MouseEvent) {
	b.hovered = true
	swapViz(b.innerRect, b.hoverRect)
	b.Refresh()
}

// MouseMoved is a hook that is called if the mouse pointer moved over the element.
func (b *ecoBttn) MouseMoved(*desktop.MouseEvent) {}

// MouseOut is a hook that is called if the mouse pointer leaves the element.
func (b *ecoBttn) MouseOut() {
	b.hovered = false
	swapViz(b.hoverRect, b.innerRect)
	b.Refresh()
}

func (b *ecoBttn) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

func swapViz(hiders, showers []fyne.CanvasObject) {
	for _, o := range showers {
		o.Show()
	}
	for _, o := range hiders {
		o.Hide()
	}
}

type ecoBttnRenderer struct {
	bttn *ecoBttn
}

// BackgroundColor returns the color that should be used to draw the background of this renderer’s widget.
//
// Deprecated: Widgets will no longer have a background to support hover and selection indication in collection widgets.
// If a widget requires a background color or image, this can be achieved by using a canvas.Rect or canvas.Image
// as the first child of a MaxLayout, followed by the rest of the widget components.
func (r *ecoBttnRenderer) BackgroundColor() color.Color {
	return color.RGBA{0, 0, 0, 255}
}

// Destroy is for internal use.
func (r *ecoBttnRenderer) Destroy() {}

// Layout is a hook that is called if the widget needs to be laid out.
// This should never call Refresh.
func (r *ecoBttnRenderer) Layout(fyne.Size) {

}

// MinSize returns the minimum size of the widget that is rendered by this renderer.
func (r *ecoBttnRenderer) MinSize() fyne.Size {
	return r.bttn.size
}

// Objects returns all objects that should be drawn.
func (r *ecoBttnRenderer) Objects() []fyne.CanvasObject {
	b := r.bttn
	objects := make([]fyne.CanvasObject, 0, 19)
	objects = append(objects, b.outerRect...)
	objects = append(objects, b.innerRect...)
	objects = append(objects, b.hoverRect...)
	objects = append(objects, b.txt)
	return objects
}

// Refresh is a hook that is called if the widget has updated and needs to be redrawn.
// This might trigger a Layout.
func (r *ecoBttnRenderer) Refresh() {}
