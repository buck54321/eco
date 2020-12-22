package ui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/driver/desktop"
	"fyne.io/fyne/widget"
)

type Orientation uint8

const (
	OrientationVertical Orientation = iota
	OrientationHorizontal
)

type Alignment uint8

const (
	// Vertical orientation. AlignLeft is both default and fallback for invalid
	// values.

	AlignLeft Alignment = iota
	AlignCenter
	AlignRight

	// Horizontal orientation. AlignMiddle is chosen when zero value or
	// otherwise invalid value is encountered.

	AlignTop
	AlignMiddle
	AlignBaseline
)

// Used for padding, margins, border-width, etc.
type FourSpec [4]int

type Justification uint8

const (
	JustifyBetween Justification = iota
	JustifyAround
	JustifyStart
	JustifyCenter
)

type EventListeners struct {
	Click    func(*fyne.PointEvent)
	MouseIn  func(*desktop.MouseEvent)
	MouseOut func()
}

type Display uint8

const (
	DisplayBlock Display = iota
	DisplayInline
)

type Position uint8

const (
	PositionRelative Position = iota
	PositionAbsolute
)

type Style struct {
	MinW, MaxW, MinH, MaxH, Width, Height interface{} // string percentage or integer pixels
	// BorderRadius and BorderWidth control the appearance of the border.
	// A zero BorderWidth indicates that no border should be drawn.
	// IMPORTANT: Due to limitations in our border drawing technique, Elements
	// with borders should always assign an opaque BgColor, so transparency must
	// be faked. This could be solved if fyne either 1) implements arbitrary
	// arc primitives, or 2) somehow enables a subtractive blend mode.
	BorderRadius, BorderWidth int
	Padding                   FourSpec // top, right, bottom, left
	Margins                   FourSpec // top, right, bottom, left
	BgColor                   color.Color
	BorderColor               color.Color
	Ori                       Orientation
	Justi                     Justification
	Align                     Alignment
	Listeners                 EventListeners
	Cursor                    desktop.Cursor
	Display                   Display
	Position                  Position
	// spacing doesn't really have an analogue in web layout, but it does in
	// other systems like Qt.  spacing only applies along the orientation
	// direction, and adds additional space between child elements.
	Spacing int
	// For absolutely positioned elements, these positioning instructions can
	// take precedence over the width and height fields, which is the opposite
	// sense of CSS. e.g. if you put a width = 10, left = 0, right = 0 element
	// inside of an element with width = 100, the nested element will end up
	// with width 100, not 10, since we prioritize the left/right pair over the
	// hard-coded width. TODO: Use these for relative positioning too.
	Left, Right, Top, Bottom *int
	// expandVertically should be used with caution. In particular, it will
	// probably not behave as desired when part of a column layout. It's really
	// only intended to support a "main window" like outer wrapper that holds
	// all the things, but it can be used other places too.
	ExpandVertically bool
}

func ParseChildDim(v interface{}, parentDim int) (int, error) {
	switch vt := v.(type) {
	case int:
		return vt, nil
	case string:
		if !strings.HasSuffix(vt, "%") {
			return 0, fmt.Errorf("ParseStyleDim cannot parse %q", vt)
		}
		vf, err := strconv.ParseFloat(strings.TrimSuffix(vt, "%"), 64)
		if err != nil {
			return 0, fmt.Errorf("ParseStyleDim -> ParseFloat cannot parse number from %q", vt)
		}
		// Truncating
		return int(vf / 100 * float64(parentDim)), nil
	}
	return 0, fmt.Errorf("Unknown type (%T) for ParseStyleDim", v)
}

// A Element is a widget used to hold another element or elements, and adds
// fully adjustable padding, margins, background-color,
// border width, border radius, border color, and fine-grained control over
// layout.
type Element struct {
	fyne.Container
	Style      *Style
	parsedDims struct {
		width, maxW, minW, height, maxH, minH int
	}
	kids           []fyne.CanvasObject
	obs            []fyne.CanvasObject
	size           fyne.Size
	minSize        fyne.Size
	claimed        fyne.Size
	cachedRenderer fyne.WidgetRenderer
	// Name is optional and only used for debugging.
	Name string
}

func NewElement(st *Style, kids ...fyne.CanvasObject) *Element {
	if st == nil {
		st = &Style{}
	}
	if st.BorderColor == nil {
		st.BorderColor = DefaultBorderColor
	}
	if st.BgColor == nil {
		st.BgColor = Transparent
	}
	if st.Ori == OrientationHorizontal && st.Align == 0 {
		st.Align = AlignMiddle
	}

	el := &Element{
		// Container: *fyne.NewContainerWithLayout(lyt, objects...),
		Style: st,
		kids:  kids,
	}
	// el.lyt = &elementLayout{
	// 	el: el,
	// }
	el.Refresh()
	return el
}

func (b *Element) InsertChild(o fyne.CanvasObject, idx int) {
	if idx < 0 || idx >= len(b.kids) {
		b.kids = append(b.kids, o)
	} else {
		b.kids = append(b.kids, nil)
		copy(b.kids[idx+1:], b.kids[idx:])
		b.kids[idx] = o
	}
	b.Refresh()
	canvas.Refresh(b)
}

func (b *Element) RemoveChildByIndex(idx int) bool {
	if idx < 0 || idx >= len(b.kids) {
		return false
	}
	copy(b.kids[idx:], b.kids[idx+1:])
	b.kids = b.kids[:len(b.kids)-1]
	b.Refresh()
	canvas.Refresh(b)
	return true
}

// MinSize returns the minimum size this object needs to be drawn.
func (b *Element) MinSize() fyne.Size {
	return b.minSize
}

func (b *Element) parseDims(parentSize fyne.Size) {
	parseDim := func(v interface{}, dim int) int {
		if v == nil {
			return 0
		}
		vi, err := ParseChildDim(v, dim)
		if err != nil {
			fmt.Println(err.Error())
			return 1
		}
		return vi
	}
	st := b.Style
	b.parsedDims.width = parseDim(st.Width, parentSize.Width)
	b.parsedDims.minW = parseDim(st.MinW, parentSize.Width)
	b.parsedDims.maxW = parseDim(st.MaxW, parentSize.Width)
	b.parsedDims.height = parseDim(st.Height, parentSize.Height)
	b.parsedDims.minH = parseDim(st.MinH, parentSize.Height)
	b.parsedDims.maxH = parseDim(st.MaxH, parentSize.Height)
}

// Resize resizes this object to the given size.
// This should only be called if your object is not in a container with a layout manager.
func (b *Element) Resize(sz fyne.Size) {
	if b.size == sz {
		return
	}

	b.parseDims(sz)

	b.size = sz
	b.claimed = b.minSize
	st := b.Style
	dims := &b.parsedDims
	if st.Position == PositionAbsolute {
		b.claimed = sz
	} else if st.Display == DisplayBlock {
		// block Element is greedy in x.
		b.claimed.Width = sz.Width
		if dims.width != 0 {
			b.claimed.Width = dims.width
		} else if b.claimed.Width < dims.minW {
			b.claimed.Width = dims.minW
		} else if dims.maxW != 0 && b.claimed.Width > dims.maxW {
			b.claimed.Width = dims.maxW
		}
	}
	if st.ExpandVertically {
		b.claimed.Height = sz.Height
	}
	// // Give the kids a chance to be greedy too.
	for _, o := range b.kids {
		if isNativeInline(o) || isAbsolutelyPositioned(o) {
			continue
		}
		if b.Name == "homeBox" || b.Name == "appRow" {
			fmt.Printf("-- %s resizing child %T \n", b.Name, o)
		}
		o.Resize(b.claimed)
	}
	// Recalculate our own MinSize.
	b.Refresh()
	if b.cachedRenderer != nil {
		b.cachedRenderer.Layout(b.claimed)
	}
}

// Size returns the current size of this object.
func (b *Element) Size() fyne.Size {
	return b.claimed
}

// Refresh must be called if this object should be redrawn because its inner
// state changed.
func (b *Element) Refresh() {
	for _, o := range b.kids {
		if b.Name == "homeBox" {
			if el, _ := o.(*Element); el != nil {
				fmt.Printf("--homeBox refreshing child Element %s \n", el.Name)
			} else {
				fmt.Printf("--homeBox refreshing child %T \n", o)
			}
		}
		o.Refresh()
	}

	st := b.Style
	dims := &b.parsedDims
	pt, pr, pb, pl := st.Padding[0], st.Padding[1], st.Padding[2], st.Padding[3]
	mt, mr, mb, ml := st.Margins[0], st.Margins[1], st.Margins[2], st.Margins[3]
	bw := st.BorderWidth
	kidSize := b.kidMinSize(b.kids)
	minW := kidSize.Width + pl + pr + ml + mr + 2*bw
	minH := kidSize.Height + pt + pb + mt + mb + 2*bw

	if b.Name == "hr" {
		fmt.Println("--Element.Refresh", minW, minH)
	}

	b.minSize = fyne.NewSize(minW, minH)

	if st.Position == PositionAbsolute {
		return
	}

	if st.Display == DisplayInline {
		b.claimed.Width = minW
	}

	if st.ExpandVertically { // Height set in Resize.
		return
	}

	b.claimed.Height = b.minSize.Height
	ySpace := mt + mb + 2*bw
	if dims.height != 0 {
		b.claimed.Height = dims.height + ySpace
	} else if dims.minH != 0 && b.claimed.Height < dims.minH {
		b.claimed.Height = dims.minH + ySpace
	} else if dims.maxH != 0 && b.claimed.Height > dims.maxH {
		b.claimed.Height = dims.maxH + ySpace
	}
}

func (b *Element) CreateRenderer() fyne.WidgetRenderer {
	r := &blockRenderer{
		el: b,
	}
	b.cachedRenderer = r
	// Do an initial layout to generate the background for empty
	// Elements, otherwise Objects will initially return 0, and the Element will
	// never even attempt to be rendered.
	r.Layout(r.el.claimed)
	return r
}

func (b *Element) Tapped(ev *fyne.PointEvent) {
	ears := b.Style.Listeners
	if ears.Click != nil {
		ears.Click(ev)
	}
}

func (b *Element) MouseIn(ev *desktop.MouseEvent) {
	ears := b.Style.Listeners
	if ears.MouseIn != nil {
		ears.MouseIn(ev)
	}
}

// MouseMoved is a hook that is called if the mouse pointer moved over the element.
func (b *Element) MouseMoved(ev *desktop.MouseEvent) {}

// MouseOut is a hook that is called if the mouse pointer leaves the element.
func (b *Element) MouseOut() {
	ears := b.Style.Listeners
	if ears.MouseOut != nil {
		ears.MouseOut()
	}
}

func (b *Element) Cursor() desktop.Cursor {
	return b.Style.Cursor
}

func (b *Element) SetBackgroundColor(c color.Color) {
	b.Style.BgColor = c
	if b.cachedRenderer != nil {
		b.cachedRenderer.Refresh()
		canvas.Refresh(b)
	}
}

// type elementLayout struct {
// 	el *Element
// }

func (b *Element) kidMinSize(objects []fyne.CanvasObject) fyne.Size {
	dims := &b.parsedDims
	if dims.width != 0 && dims.height != 0 {
		return fyne.NewSize(dims.width, dims.height)
	}
	minW, minH := dims.minW, dims.minH
	maxW, maxH := dims.maxW, dims.maxH
	w, h := b.actualMinSize(objects)

	if dims.width != 0 {
		w = dims.width
	} else {
		if minW != 0 && w < minW {
			w = minW
		}
		if maxW != 0 && w > maxW {
			w = maxW
		}
	}
	if dims.height != 0 {
		h = dims.height
	} else {
		if minH != 0 && h < minH {
			h = minH
		}
		if maxH != 0 && h > maxH {
			h = maxH
		}
	}
	return fyne.NewSize(w, h)
}

func (b *Element) actualMinSize(objects []fyne.CanvasObject) (w, h int) {
	if b.Style.Ori == OrientationVertical {
		return b.minSizeColumn(objects)
	}
	w, h = b.minSizeRow(objects)
	return w, h
}

func (b *Element) Layout(objects []fyne.CanvasObject, containerSize fyne.Size) {
	n := len(objects)
	if n == 0 {
		return
	}

	minW, minH := b.actualMinSize(objects)
	st := b.Style
	if st.Ori == OrientationHorizontal {
		b.layoutRow(objects, minW)
		return
	}

	b.layoutColumn(objects, minH)
}

func (b *Element) minSizeRow(objects []fyne.CanvasObject) (w, h int) {
	var spacing int
	for _, o := range objects {
		if !o.Visible() || isAbsolutelyPositioned(o) {
			continue
		}
		childSize := o.MinSize()
		if childSize.Height > h {
			h = spacing + childSize.Height
		}

		w += spacing + childSize.Width
		spacing = b.Style.Spacing
	}
	return w, h
}

func (b *Element) minSizeColumn(objects []fyne.CanvasObject) (w, h int) {
	var spacing int
	for _, o := range objects {
		if !o.Visible() || isAbsolutelyPositioned(o) {
			continue
		}
		childSize := o.MinSize()

		if childSize.Width > w {
			w = spacing + childSize.Width
		}
		h += spacing + childSize.Height
		spacing = b.Style.Spacing
	}
	return w, h
}

func (b *Element) extraSpace() (x, y int) {
	st := b.Style
	pt, pr, pb, pl := st.Padding[0], st.Padding[1], st.Padding[2], st.Padding[3]
	mt, mr, mb, ml := st.Margins[0], st.Margins[1], st.Margins[2], st.Margins[3]
	bw := st.BorderWidth
	x = 2*bw + ml + pl + mr + pr
	y = 2*bw + mt + pt + mb + pb
	return
}

func (b *Element) availDims() (w, h int) {
	xSpace, ySpace := b.extraSpace()
	return b.claimed.Width - xSpace, b.claimed.Height - ySpace
}

func (b *Element) layoutRow(objects []fyne.CanvasObject, minW int) {
	claimed := b.claimed
	st := b.Style
	justi, align := st.Justi, st.Align
	flowingObs, _ := layoutItems(objects)
	n := len(flowingObs)
	availWidth, availHeight := b.availDims()
	spacing := st.Spacing

	var padding int
	switch justi {
	case JustifyStart, JustifyCenter:
	case JustifyAround:
		if n > 0 {
			padding = (availWidth - minW) / n
		}
	case JustifyBetween:
		if n > 1 {
			padding = (availWidth - minW) / (n - 1)
		}
	}

	pos := fyne.NewPos(0, 0)
	switch justi {
	case JustifyAround:
		pos = fyne.NewPos(padding/2, 0)
	case JustifyCenter:
		pos = fyne.NewPos((availWidth-minW)/2, 0)
	}

	for _, o := range flowingObs {
		if !o.Visible() {
			continue
		}
		size := o.Size()

		var y int
		switch align {
		case AlignMiddle:
			y = (availHeight - size.Height) / 2
		case AlignBaseline:
			y = availHeight - size.Height
		}

		pos.Y = y

		if b.Name == "lbl" {
			fmt.Printf("-- %T: pos = %v, claimed = %v, size = %v, n = %v \n", o, pos, claimed, size, len(flowingObs))
		}

		o.Move(pos)

		// We're choosing center alignment here, but we could also implement
		// other alignments.
		pos.X += size.Width + padding + spacing
	}
}

func (b *Element) layoutColumn(objects []fyne.CanvasObject, minH int) {
	claimed := b.claimed
	st := b.Style
	justi, align := st.Justi, st.Align
	flowingObs, _ := layoutItems(objects)
	n := len(flowingObs)
	availWidth, availHeight := b.availDims()
	spacing := st.Spacing

	var padding int
	switch justi {
	case JustifyStart, JustifyCenter:
	case JustifyAround:
		if n > 0 {
			padding = (availHeight - minH) / n
		}
	case JustifyBetween:
		if n > 1 {
			padding = (availHeight - minH) / (n - 1)
		}
	}

	pos := fyne.NewPos(0, 0)
	switch justi {
	case JustifyAround:
		pos = fyne.NewPos(0, padding/2)
	case JustifyCenter:
		pos = fyne.NewPos(0, (availHeight-minH)/2)
	}

	for _, o := range flowingObs {
		if !o.Visible() {
			continue
		}
		size := o.Size()

		var x int
		switch align {
		case AlignCenter:
			x = (availWidth - size.Width) / 2
		case AlignRight:
			x = availWidth - size.Width
		}

		pos.X = x

		o.Move(pos)

		if b.Name == "datum" {
			fmt.Printf("-- %T, position = %v, claimed = %v, size = %v, padding = %v \n", o, pos, claimed, size, padding)
		}

		pos.Y += size.Height + padding + spacing
	}
}

func (b *Element) positionAbsolutely(obs []fyne.CanvasObject) {
	if len(obs) == 0 {
		return
	}
	st, claimed := b.Style, b.claimed
	mt, mr, mb, ml := st.Margins[0], st.Margins[1], st.Margins[2], st.Margins[3]
	bw := st.BorderWidth
	absH := claimed.Height - mb - mt - 2*bw
	absW := claimed.Width - mr - ml - 2*bw

	for _, o := range obs {
		oSt := o.(*Element).Style // Already know this is an *Element because it was filtered through layoutItems.
		t, r, b, l := oSt.Top, oSt.Right, oSt.Bottom, oSt.Left
		sz := o.MinSize()
		pos := fyne.NewPos(ml+bw, mt+bw)
		if l != nil {
			pos.X = *l + ml + bw
			if r != nil {
				sz.Width = absW - *r - pos.X
			}
		} else if r != nil {
			pos.X = claimed.Width - *r - sz.Width - mr - bw
		}
		if t != nil {
			pos.Y = *t + mt + bw
			if b != nil {
				sz.Height = absH - *b - pos.Y
			}
		} else if b != nil {
			pos.Y = claimed.Height - *b - sz.Height - mb - bw
		}

		o.Move(pos)
		o.Resize(sz)
	}
}

type blockRenderer struct {
	el *Element
}

// BackgroundColor returns the color that should be used to draw the background of this renderer’s widget.
//
// Deprecated: Widgets will no longer have a background to support hover and selection indication in collection widgets.
// If a widget requires a background color or image, this can be achieved by using a canvas.Rect or canvas.Image
// as the first child of a MaxLayout, followed by the rest of the widget components.
func (r *blockRenderer) BackgroundColor() color.Color {
	return Transparent
}

// Destroy is for internal use.
func (r *blockRenderer) Destroy() {}

// Layout is a hook that is called if the widget needs to be laid out.
// This should never call Refresh.
func (r *blockRenderer) Layout(sz fyne.Size) {
	st := r.el.Style
	pt, pr, pb, pl := st.Padding[0], st.Padding[1], st.Padding[2], st.Padding[3]
	mt, mr, mb, ml := st.Margins[0], st.Margins[1], st.Margins[2], st.Margins[3]
	bw, br, brdc, bgc := st.BorderWidth, st.BorderRadius, st.BorderColor, st.BgColor
	xSpace := 2*bw + ml + pl + mr + pr
	ySpace := 2*bw + mt + pt + mb + pb
	lSpace := bw + ml + pl
	tSpace := bw + mt + pt

	// The elementLayout takes care of the sizing within the kids' space. We need
	// to offset the within our own margins, padding, and borders.
	kidBoxSize := r.el.claimed.Subtract(fyne.NewSize(xSpace, ySpace))
	flowingKids, positionedKids := layoutItems(r.el.kids)
	r.el.Layout(flowingKids, kidBoxSize)
	r.el.positionAbsolutely(positionedKids)

	kidOffset := fyne.NewPos(lSpace, tSpace)

	if r.el.Name == "hr" {
		fmt.Println("--blockRenderer.Layout ", r.el.claimed, kidBoxSize)
	}

	applyOffset(flowingKids, kidOffset)

	x, y := ml, mt
	workingSize := r.el.claimed.Subtract(fyne.NewSize(ml+mr, mt+mb))
	obs := make([]fyne.CanvasObject, 0)
	if st.BorderWidth > 0 {
		if br > 0 {
			obs = append(obs, roundedRectangle(workingSize.Width, workingSize.Height,
				x, y, br, float32(br*2), brdc)...)
		} else {
			rect := canvas.NewRectangle(brdc)
			rect.Resize(workingSize)
			obs = append(obs, rect)
			rect.Move(fyne.NewPos(x, y))
		}
		x += bw
		y += bw
		workingSize = workingSize.Subtract(fyne.NewSize(bw*2, bw*2))
	}

	if bgc != Transparent {
		if br > 0 {
			obs = append(obs, roundedRectangle(workingSize.Width, workingSize.Height,
				x, y, br, float32(br*2), bgc)...)
		} else {
			rect := canvas.NewRectangle(bgc)
			rect.Resize(workingSize)
			rect.Move(fyne.NewPos(x, y))
			obs = append(obs, rect)
		}
	}
	r.el.obs = obs
}

func applyOffset(obs []fyne.CanvasObject, offset fyne.Position) {
	for _, ob := range obs {
		ob.Move(ob.Position().Add(offset))
	}
}

// MinSize returns the minimum size of the widget that is rendered by this renderer.
func (r *blockRenderer) MinSize() fyne.Size {
	return r.el.claimed
}

// Objects returns all objects that should be drawn.
func (r *blockRenderer) Objects() []fyne.CanvasObject {
	// if r.el.Name == "mainWin" {
	// 	fmt.Println("--blockRenderer.Objects", len(append(r.el.obs, r.el.kids...)))
	// }

	return append(r.el.obs, r.el.kids...)
}

// Refresh is a hook that is called if the widget has updated and needs to be redrawn.
// This might trigger a Layout.
func (r *blockRenderer) Refresh() {
	r.el.Refresh()
}

// These elements won't be Resize'd recursively.
func isNativeInline(o fyne.CanvasObject) bool {
	switch o.(type) {
	case *canvas.Image, *canvas.Text, *widget.Label:
		return true
	}
	return false
}

func isAbsolutelyPositioned(o fyne.CanvasObject) bool {
	el, ok := o.(*Element)
	return ok && el.Style.Position == PositionAbsolute
}
