package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/driver/desktop"
	"fyne.io/fyne/theme"
	"fyne.io/fyne/widget"
	"github.com/decred/slog"
	"github.com/nfnt/resize"
)

const (
	regularFont = "SourceSans3-Regular.ttf"
	boldFont    = "source-sans-pro-semibold.ttf"

	introductionText = "For the best security and the full range of Decred services, you'll want to sync the full blockchain, which will use around 5 GB of disk space. If you're only interested in basic wallet functionality, you may choose to sync in SPV mode, which will be very fast and use about 100 MB of disk space."
)

var (
	log = slog.NewBackend(os.Stdout).Logger("GUI")

	bttnColor               = stringToColor("#005")
	bgColor                 = stringToColor("#000008")
	transparent             = stringToColor("#0000")
	defaultButtonColor      = stringToColor("#003")
	defaultButtonHoverColor = stringToColor("#005")
	buttonColor2            = stringToColor("#001a08")
	buttonHoverColor2       = stringToColor("#00251a")
	textColor               = stringToColor("#e1e1e1")
	cursorColor             = stringToColor("#2970fe")
	focusColor              = cursorColor

	logoRsrc = mustLoadStaticResource("eco-logo.png")
)

func main() {
	// a := app.New()
	// fyne.CurrentApp().Settings().SetTheme(newDefaultTheme())
	// w := a.NewWindow("Hello There")
	// w.Resize(fyne.NewSize(1024, 768))

	// intro := widget.NewLabel("")
	// intro.Wrapping = fyne.TextWrapWord
	// intro.BaseWidget.Resize(fyne.NewSize(450, 0))
	// intro.Text = introductionText

	// entry := &betterEntry{Entry: &widget.Entry{}, w: 450}
	// entry.PlaceHolder = "set your password"
	// entry.Password = true
	// entry.ExtendBaseWidget(entry)

	// bttn1 := newEcoBttn(&bttnOpts{
	// 	bgColor:    buttonColor2,
	// 	hoverColor: buttonHoverColor2,
	// }, "Full Sync", func() {
	// 	fmt.Println("Ooh. That felt nice.")
	// })

	// bttn2 := newEcoBttn(nil, "Lite Mode (SPV)", func() {
	// 	fmt.Println("That felt so so.")
	// })

	// // bttn.Importance = widget.HighImportance
	// logo := newSizedImage(logoRsrc, 0, 40)

	// w.SetContent(container.NewVBox(container.NewCenter(fyne.NewContainerWithLayout(
	// 	&ecoColumn{
	// 		spacing:  30,
	// 		yPadding: 20,
	// 	},
	// 	logo,
	// 	newLabelWithWidth(intro, 450),
	// 	entry,
	// 	newFlexRow(&flexOpts{
	// 		w:             450,
	// 		justification: justifyAround,
	// 	}, bttn1, bttn2),
	// ))))

	// w.ShowAndRun()
	gui := NewGUI()
	gui.Run()
}

type defaultTheme struct {
	fyne.Theme
	regularFont *fyne.StaticResource
	boldFont    *fyne.StaticResource
}

func newDefaultTheme() *defaultTheme {
	return &defaultTheme{
		Theme:       theme.DarkTheme(),
		regularFont: mustLoadStaticResource(regularFont),
		boldFont:    mustLoadStaticResource(boldFont),
	}
}

func (t *defaultTheme) BackgroundColor() color.Color {
	return bgColor
}

func (t *defaultTheme) Padding() int {
	return 5
}

func (t *defaultTheme) ButtonColor() color.Color {
	return bttnColor
}

func (t *defaultTheme) TextFont() fyne.Resource {
	return t.regularFont
}

func (t *defaultTheme) TextBoldFont() fyne.Resource {
	return t.boldFont
}

func (t *defaultTheme) TextColor() color.Color {
	return textColor
}

func (t *defaultTheme) TextSize() int {
	return 15
}

// func (t *defaultTheme) ShadowColor() color.Color {
// 	return transparent
// }

func (t *defaultTheme) FocusColor() color.Color {
	return focusColor
}

// DisabledButtonColor() color.Color
// // Deprecated: Hyperlinks now use the primary color for consistency.
// HyperlinkColor() color.Color
// DisabledTextColor() color.Color
// // Deprecated: Icons now use the text colour for consistency.
// IconColor() color.Color
// // Deprecated: Disabled icons match disabled text color for consistency.
// DisabledIconColor() color.Color
// PlaceHolderColor() color.Color
// PrimaryColor() color.Color
// HoverColor() color.Color
// FocusColor() color.Color
// ScrollBarColor() color.Color
// ShadowColor() color.Color

// TextSize() int
// TextFont() Resource

// TextItalicFont() Resource
// TextBoldItalicFont() Resource
// TextMonospaceFont() Resource

// IconInlineSize() int
// ScrollBarSize() int
// ScrollBarSmallSize() int

func mustLoadStaticResource(rsc string) *fyne.StaticResource {
	b, err := ioutil.ReadFile(filepath.Join("static", rsc))
	if err != nil {
		panic("error loading font " + err.Error())
	}

	return &fyne.StaticResource{
		StaticName:    rsc,
		StaticContent: b,
	}
}

func stringToColor(s string) color.Color {
	var c color.NRGBA
	var err error
	if strings.HasPrefix(s, "#") {
		s = s[1:]
	}
	n := len(s)
	if n == 6 {
		c.A = 0xFF
		_, err = fmt.Sscanf(s, "%02x%02x%02x", &c.R, &c.G, &c.B)
	} else if n == 8 {
		_, err = fmt.Sscanf(s, "%02x%02x%02x%02x", &c.R, &c.G, &c.B, &c.A)
	} else if n == 3 || n == 4 {
		if n == 3 {
			c.A = 0xFF
			_, err = fmt.Sscanf(s, "%01x%01x%01x", &c.R, &c.G, &c.B)
		} else {
			_, err = fmt.Sscanf(s, "%01x%01x%01x%01x", &c.R, &c.G, &c.B, &c.A)
			c.A |= (c.A << 4)
		}
		c.R |= (c.R << 4)
		c.G |= (c.G << 4)
		c.B |= (c.B << 4)
	}
	if err != nil {
		panic("Error converting color string '" + s + "' : " + err.Error())
	}
	return c
}

func newSizedImage(rsc *fyne.StaticResource, w, h uint) *canvas.Image {
	img, _, err := image.Decode(bytes.NewReader(rsc.StaticContent))
	if err != nil {
		panic("image err " + err.Error())
	}
	ogSize := img.Bounds().Size()
	imgW, imgH := uint(ogSize.X), uint(ogSize.Y)
	needsResize := (w != 0 && w != imgW) || (h != 0 && h != imgH)

	aspect := float64(imgW) / float64(imgH)
	if w == 0 {
		if h == 0 {
			w = imgW
			h = imgH
		} else {
			w = uint(math.Round(aspect * float64(h)))
		}
	} else if h == 0 {
		h = uint(math.Round(float64(w) / aspect))
	}

	if needsResize {
		img = resize.Resize(w, h, img, resize.Lanczos3)
	}

	fyneImg := canvas.NewImageFromImage(img)
	fyneImg.FillMode = canvas.ImageFillOriginal
	return fyneImg
}

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
	if sz.Width == b.w {
		return sz
	}
	return fyne.NewSize(b.w, sz.Height)
}

type ecoColumn struct {
	spacing  int
	yPadding int
}

func (lyt *ecoColumn) MinSize(objects []fyne.CanvasObject) fyne.Size {
	w, h := 0, lyt.yPadding*2 // top and bottom padding
	for _, o := range objects {
		childSize := o.MinSize()

		if childSize.Width > w {
			w = childSize.Width
		}
		h += childSize.Height
	}
	n := len(objects)
	if n > 1 {
		h += (n - 1) + lyt.spacing
	}
	return fyne.NewSize(w, h)
}

func (lyt *ecoColumn) Layout(objects []fyne.CanvasObject, containerSize fyne.Size) {
	lytSize := lyt.MinSize(objects)
	pos := fyne.NewPos(0, lyt.yPadding)
	for _, o := range objects {
		size := o.MinSize()
		padX := (lytSize.Width - size.Width) / 2

		o.Resize(size)
		o.Move(pos.Add(fyne.NewPos(pos.X+padX, pos.X)))

		pos = pos.Add(fyne.NewPos(0, size.Height+lyt.spacing))
	}
}

type flexJustification uint8

const (
	justifyBetween flexJustification = iota
	justifyAround
)

type flexOpts struct {
	w, h          int
	justification flexJustification
}

type flexRow struct {
	opts *flexOpts
}

func newFlexRow(opts *flexOpts, objects ...fyne.CanvasObject) *fyne.Container {
	return fyne.NewContainerWithLayout(&flexRow{
		opts: opts,
	}, objects...)
}

func (lyt *flexRow) MinSize(objects []fyne.CanvasObject) fyne.Size {
	w, h := lyt.actualMinSize(objects)
	if lyt.opts.w != 0 {
		w = lyt.opts.w
	}
	if lyt.opts.h != 0 {
		h = lyt.opts.h
	}
	return fyne.NewSize(w, h)
}

func (lyt *flexRow) actualMinSize(objects []fyne.CanvasObject) (w, h int) {
	for _, o := range objects {
		childSize := o.MinSize()

		if childSize.Height > h {
			h = childSize.Height
		}
		w += childSize.Width
	}
	return w, h
}

func (lyt *flexRow) Layout(objects []fyne.CanvasObject, containerSize fyne.Size) {
	n := len(objects)
	if n == 0 {
		return
	}

	minW, _ := lyt.actualMinSize(objects)

	j := lyt.opts.justification

	var padding int
	if n > 1 {
		divisor := (n - 1)
		if j == justifyAround {
			divisor = n
		}
		padding = (containerSize.Width - minW) / divisor
	}

	pos := fyne.NewPos(0, 0)
	if j == justifyAround {
		pos = fyne.NewPos(padding/2, 0)
	}

	for _, o := range objects {
		size := o.MinSize()
		o.Resize(size)
		o.Move(pos)
		pos = pos.Add(fyne.NewPos(size.Width+padding, 0))
	}
}

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

func newEcoBttn(opts *bttnOpts, text string, click func()) *ecoBttn {
	if opts == nil {
		opts = &bttnOpts{}
	}
	nonzero := func(this, that int) int {
		if this == 0 {
			return that
		}
		return this
	}
	borderRadius := 2
	borderWidth := 1
	borderColor := stringToColor("#333")
	fontSize := nonzero(opts.fontSize, 14)
	paddingX := nonzero(opts.paddingX, 20)
	paddingY := nonzero(opts.paddingY, 10)
	clr := opts.bgColor
	if clr == nil {
		clr = defaultButtonColor
	}
	hoverClr := opts.hoverColor
	if hoverClr == nil {
		hoverClr = defaultButtonHoverColor
	}

	txt := canvas.NewText(text, textColor)
	txt.TextSize = fontSize
	txt.TextStyle.Bold = true
	txtBox := txt.MinSize()
	txt.Move(fyne.NewPos(paddingX+borderWidth, paddingY+borderWidth))
	outerX := txtBox.Width + 2*paddingX + 2*borderWidth
	outerY := txtBox.Height + 2*paddingY + 2*borderWidth
	innerX := outerX - 2*borderWidth
	innerY := outerY - 2*borderWidth

	line := func(startX, startY, endX, endY int, c color.Color) *canvas.Line {
		ln := canvas.NewLine(c)
		ln.StrokeWidth = float32(borderRadius) * 2
		ln.Move(fyne.NewPos(startX, startY))
		ln.Resize(fyne.NewSize(endX-startX, endY-startY))
		return ln
	}

	corner := func(x, y int, c color.Color) *canvas.Circle {
		circ := canvas.NewCircle(c)
		circ.Move(fyne.NewPos(x-borderRadius, y-borderRadius))
		circ.Resize(fyne.NewSize(borderRadius*2, borderRadius*2))
		return circ
	}

	// There are nine shapes that make up a rounded rectangle. 4 corner circles,
	// 4 side lines, and 1 center rect
	roundedRect := func(w, h, offsetX, offsetY int, c color.Color) []fyne.CanvasObject {
		x, y := borderRadius+offsetX, borderRadius+offsetY
		innerH := h - 2*borderRadius
		innerW := w - 2*borderRadius
		left := line(x, y, x, y+innerH, c)
		topLeft := corner(x, y, c)
		top := line(x, x, y+innerW, y, c)
		x += innerW
		topRight := corner(x, y, c)
		right := line(x, y, x, y+innerH, c)
		y += innerH
		bottomRight := corner(x, y, c)
		x = borderRadius + offsetX
		bottom := line(x, y, x+innerW, y, c)
		bottomLeft := corner(x, y, c)

		fill := canvas.NewRectangle(c)
		fill.Move(fyne.NewPos(borderRadius, borderRadius))
		fill.Resize(fyne.NewSize(innerW, innerH))

		return []fyne.CanvasObject{
			left, topLeft, top, topRight, right, bottomRight, bottom, bottomLeft, fill,
		}
	}

	bttn := &ecoBttn{
		size:      fyne.NewSize(outerX, outerY),
		outerRect: roundedRect(outerX, outerY, 0, 0, borderColor),
		innerRect: roundedRect(innerX, innerY, borderWidth, borderWidth, clr),
		hoverRect: roundedRect(innerX, innerY, borderWidth, borderWidth, hoverClr),
		txt:       txt,
		click:     click,
	}
	swapViz(bttn.hoverRect, nil)
	return bttn
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

type betterEntryRenderer struct {
	fyne.WidgetRenderer
	baseObjects []fyne.CanvasObject
	be          *betterEntry
}

func (r *betterEntryRenderer) Objects() []fyne.CanvasObject {
	objects := r.WidgetRenderer.Objects()

	findRect := func(obs []fyne.CanvasObject) (i int, found bool) {
		for i = range obs {
			if _, ok := objects[i].(*canvas.Rectangle); ok {
				found = true
				return
			}
		}
		return
	}

	if len(r.baseObjects) == 0 && (len(objects) == 4 || (len(objects) == 5 && r.be.Password)) { // Password fields have and extra element at the end, making 5
		// It's probably at index 1, but there can be selections first too, so
		// just find the first rectangle instead.
		i, found := findRect(objects)
		if found {
			r.baseObjects = make([]fyne.CanvasObject, len(objects)-1)
			copy(r.baseObjects, objects[:i])
			copy(r.baseObjects[i:], objects[i+1:])
		}

		// The next rectangle is the cursor. Set the fill color.
		i, found = findRect(r.baseObjects)
		if found {
			cursor, ok := r.baseObjects[i].(*canvas.Rectangle)
			if ok {
				cursor.FillColor = cursorColor
				sz := cursor.Size()
				cursor.Resize(fyne.NewSize(sz.Width, sz.Height-(&defaultTheme{}).Padding()))
			}
		}
	}

	// // Locate the text nodes and reposition. The base type is unexported, but
	// // we can identify it because it will be a Stringer.
	// for _, o := range r.baseObjects {
	// 	if _, ok := o.(fmt.Stringer); ok {
	// 		fmt.Println("--moving the text")
	// 		o.Move(fyne.Position{})
	// 	}
	// }

	// for i, o := range r.baseObjects {
	// 	fmt.Printf("--%d: %T", i, o)
	// }
	// fmt.Println("")

	selections := make([]fyne.CanvasObject, 0)
	if len(r.baseObjects) > 0 && len(objects) > 4 {
		// pop selections
		for _, o := range objects {
			if _, ok := o.(*canvas.Rectangle); !ok {
				break
			}
			selections = append(selections, o)
		}
	}
	if len(selections) > 0 {
		selections = selections[:len(selections)-1]
	}

	return append(selections, r.baseObjects...)
}

func (r *betterEntryRenderer) BackgroundColor() color.Color {
	return stringToColor("#111")
}

type betterEntry struct {
	*widget.Entry
	w, h           int
	cachedRenderer fyne.WidgetRenderer
}

func (be *betterEntry) CreateRenderer() fyne.WidgetRenderer {
	return &betterEntryRenderer{WidgetRenderer: be.Entry.CreateRenderer(), be: be}
}

func (be *betterEntry) MinSize() fyne.Size {
	baseSz := be.Entry.MinSize()
	baseSz.Width = be.w
	if be.h == 0 {
		be.h = baseSz.Height - 3
	}
	baseSz.Height = be.h
	return baseSz
}
