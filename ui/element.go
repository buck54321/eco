package ui

import (
	"fmt"
	"image/color"

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
	// Horizontal alignments. left typically chosen when outside of valid range.
	AlignLeft Alignment = iota
	AlignCenter
	AlignRight
	// Vertical alignments. middle typically chosen when outside of valid range.
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
	MinW, MaxW, MinH, MaxH, Width, Height int
	BorderRadius, BorderWidth             int
	Padding                               FourSpec // top, right, bottom, left
	Margins                               FourSpec // top, right, bottom, left
	BgColor                               color.Color
	BorderColor                           color.Color
	Ori                                   Orientation
	Justi                                 Justification
	Align                                 Alignment
	Listeners                             EventListeners
	Cursor                                desktop.Cursor
	Display                               Display
	Position                              Position
	// scroll                                elementScroll
	// spacing doesn't really have an analogue in web layout, but it does in
	// other systems like Qt.  spacing only applies along the orientation
	// direction, and adds additional space between child elements.
	Spacing int
	// These positioning instructions can take precedence over the width and
	// height fields, which is the opposite sense of CSS. e.g. if you put a
	// width = 10, left = 0, right = 0 element inside of an element with
	// width = 100, the nested element will end up with width 100, not 10, since
	// we prioritize the left/right pair over the hard-coded width.
	Left, Right, Top, Bottom *int
	// expandVertically should be used with caution. In particular, it will
	// probably not behave as desired when part of a column layout. It's really
	// only intended to support a "main window" like outer wrapper that holds
	// all the things, but it can be used other places too.
	ExpandVertically bool
}

// A Element is a widget used to hold another element or elements, and adds
// fully adjustable padding, margins, background-color,
// border width, border radius, border color, and fine-grained control over
// layout.
type Element struct {
	fyne.Container
	Style          *Style
	lyt            *elementLayout
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
	el.lyt = &elementLayout{
		el: el,
	}
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

// MinSize returns the minimum size this object needs to be drawn.
func (b *Element) MinSize() fyne.Size {
	return b.minSize
}

// Resize resizes this object to the given size.
// This should only be called if your object is not in a container with a layout manager.
func (b *Element) Resize(sz fyne.Size) {
	if b.size == sz {
		return
	}

	if b.Name == "homeBox" || b.Name == "appRow" {
		fmt.Printf("--Element.Resize for %s: %v \n", b.Name, sz)
	}

	b.size = sz
	b.claimed = b.minSize
	st := b.Style
	if st.Position == PositionAbsolute {
		b.claimed = sz
	} else if st.Display == DisplayBlock {
		// block Element is greedy in x.
		b.claimed.Width = sz.Width
		if st.Width != 0 {
			b.claimed.Width = st.Width
		} else if b.claimed.Width < st.MinW {
			b.claimed.Width = st.MinW
		} else if st.MaxW != 0 && b.claimed.Width > st.MaxW {
			b.claimed.Width = st.MaxW
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
	pt, pr, pb, pl := st.Padding[0], st.Padding[1], st.Padding[2], st.Padding[3]
	mt, mr, mb, ml := st.Margins[0], st.Margins[1], st.Margins[2], st.Margins[3]
	bw := st.BorderWidth
	kidSize := b.lyt.MinSize(b.kids)
	minW := kidSize.Width + pl + pr + ml + mr + 2*bw
	minH := kidSize.Height + pt + pb + mt + mb + 2*bw

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
	if st.Height != 0 {
		b.claimed.Height = st.Height
	} else if st.MinH != 0 && b.claimed.Height < st.MinH {
		b.claimed.Height = st.MinH
	} else if st.MaxH != 0 && b.claimed.Height > st.MaxH {
		b.claimed.Height = st.MaxH
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

type elementLayout struct {
	el *Element
}

func (lyt *elementLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	st := lyt.el.Style
	if st.Width != 0 && st.Height != 0 {
		return fyne.NewSize(st.Width, st.Height)
	}
	minW, minH := st.MinW, st.MinH
	maxW, maxH := st.MaxW, st.MaxH
	w, h := lyt.actualMinSize(objects)

	if st.Width != 0 {
		w = st.Width
	} else {
		if minW != 0 && w < minW {
			w = minW
		}
		if maxW != 0 && w > maxW {
			w = maxW
		}
	}
	if st.Height != 0 {
		h = st.Height
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

func (lyt *elementLayout) actualMinSize(objects []fyne.CanvasObject) (w, h int) {
	if lyt.el.Style.Ori == OrientationVertical {
		return lyt.minSizeColumn(objects)
	}
	w, h = lyt.minSizeRow(objects)
	return w, h
}

func (lyt *elementLayout) Layout(objects []fyne.CanvasObject, containerSize fyne.Size) {
	n := len(objects)
	if n == 0 {
		return
	}

	minW, minH := lyt.actualMinSize(objects)
	st := lyt.el.Style
	if st.Ori == OrientationHorizontal {
		lyt.layoutRow(objects, minW)
		return
	}

	lyt.layoutColumn(objects, minH)
}

func (lyt *elementLayout) minSizeRow(objects []fyne.CanvasObject) (w, h int) {
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
		spacing = lyt.el.Style.Spacing
	}
	return w, h
}

func (lyt *elementLayout) minSizeColumn(objects []fyne.CanvasObject) (w, h int) {
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
		spacing = lyt.el.Style.Spacing
	}
	return w, h
}

func (lyt *elementLayout) layoutRow(objects []fyne.CanvasObject, minW int) {
	claimed := lyt.el.claimed
	st := lyt.el.Style
	justi, align := st.Justi, st.Align
	flowingObs, _ := layoutItems(objects)
	n := len(flowingObs)
	pr, pl := st.Padding[1], st.Padding[3]
	mr, ml := st.Margins[1], st.Margins[3]
	bw := st.BorderWidth
	xSpace := 2*bw + ml + pl + mr + pr
	availWidth := claimed.Width - xSpace
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
			y = (claimed.Height - size.Height) / 2
		case AlignBaseline:
			y = claimed.Height - size.Height
		}

		pos.Y = y

		if lyt.el.Name == "appRow" {
			fmt.Printf("-- %T: pos = %v, claimed = %v, size = %v, n = %v \n", o, pos, claimed, size, len(flowingObs))
		}

		o.Move(pos)

		// We're choosing center alignment here, but we could also implement
		// other alignments.
		pos.X += size.Width + padding + spacing
	}
}

func (lyt *elementLayout) layoutColumn(objects []fyne.CanvasObject, minH int) {

	claimed := lyt.el.claimed
	st := lyt.el.Style
	justi, align := st.Justi, st.Align
	flowingObs, _ := layoutItems(objects)
	n := len(flowingObs)
	pt, pb := st.Padding[0], st.Padding[2]
	mt, mb := st.Margins[0], st.Margins[2]
	bw := st.BorderWidth
	ySpace := 2*bw + mt + pt + mb + pb
	availHeight := claimed.Height - ySpace
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
		// size := o.MinSize()
		// o.Resize(size)
		size := o.Size()

		var x int
		switch align {
		case AlignCenter:
			x = (claimed.Width - size.Width) / 2
		case AlignRight:
			x = claimed.Width - size.Width
		}

		pos.X = x

		o.Move(pos)

		if lyt.el.Name == "mainWin" {
			fmt.Printf("-- %T, position = %v, claimed.Width = %v, size = %v, padding = %v \n", o, pos, claimed.Width, size, padding)
		}

		pos.Y += size.Height + padding + spacing
	}
}

func (lyt *elementLayout) positionAbsolutely(obs []fyne.CanvasObject) {
	if len(obs) == 0 {
		return
	}
	st, claimed := lyt.el.Style, lyt.el.claimed
	mt, mr, mb, ml := st.Margins[0], st.Margins[1], st.Margins[2], st.Margins[3]
	bw := st.BorderWidth
	absH := claimed.Height - mb - mt - 2*bw
	absW := claimed.Width - mr - ml - 2*bw

	for _, o := range obs {
		oSt := o.(*Element).Style // Already know this is an *Element because it was filtered through layoutItems.
		t, r, b, l := oSt.Top, oSt.Right, oSt.Bottom, oSt.Left
		sz := o.MinSize()
		pos := fyne.NewPos(0, 0)
		if l != nil {
			pos.X = *l
			if r != nil {
				sz.Width = absW - *r - pos.X
			}
		} else if r != nil {
			pos.X = claimed.Width - *r - sz.Width - mr - bw
		}
		if t != nil {
			pos.Y = *t
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
	r.el.lyt.Layout(flowingKids, kidBoxSize)
	r.el.lyt.positionAbsolutely(positionedKids)

	kidOffset := fyne.NewPos(lSpace, tSpace)
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
			rect.Move(fyne.NewPos(bw, bw))
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
