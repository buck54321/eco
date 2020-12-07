package main

import (
	"fmt"
	"image/color"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/driver/desktop"
	"fyne.io/fyne/widget"
)

type eventListeners struct {
	click    func(*fyne.PointEvent)
	mouseIn  func(*desktop.MouseEvent)
	mouseOut func()
}

type elementStyle struct {
	minW, maxW, minH, maxH, width, height int
	borderRadius, borderWidth             int
	padding                               borderSpecs // top, right, bottom, left
	margins                               borderSpecs
	bgColor                               color.Color
	borderColor                           color.Color
	bold                                  bool
	ori                                   orientation
	justi                                 flexJustification
	align                                 alignment
	listeners                             eventListeners
	cursor                                desktop.Cursor
	display                               elementDisplay
	position                              elementPosition
	scroll                                elementScroll
	// spacing doesn't really have an analogue in web layout, but it does in
	// other systems like Qt.  spacing only applies along the orientation
	// direction, and adds additional space between child elements.
	spacing int
	// These positioning instructions can take precedence over the width and
	// height fields, which is the opposite sense of CSS. e.g. if you put a
	// width = 10, left = 0, right = 0 element inside of an element with
	// width = 100, the nested element will end up with width 100, not 10, since
	// we prioritize the left/right pair over the hard-coded width.
	left, right, top, bottom *int
}

type elementDisplay uint8

const (
	displayBlock = iota
	displayInline
)

type elementPosition uint8

const (
	positionRelative = iota
	positionAbsolute
)

type elementScroll uint8 // Bitmask

const (
	scrollNope = 1 << iota
	scrollVertical
	scrollHorizontal
)

// A Element is a widget used to hold another element or elements, and adds
// fully adjustable padding, margins, background-color,
// border width, border radius, border color, and fine-grained control over
// layout.
type Element struct {
	fyne.Container
	cfg            *elementStyle
	lyt            *elementLayout
	kids           []fyne.CanvasObject
	obs            []fyne.CanvasObject
	size           fyne.Size
	minSize        fyne.Size
	claimed        fyne.Size
	cachedRenderer fyne.WidgetRenderer
	// name is optional and only used for debugging.
	name string
}

func newElement(cfg *elementStyle, kids ...fyne.CanvasObject) *Element {
	if cfg.borderColor == nil {
		cfg.borderColor = defaultBorderColor
	}
	if cfg.bgColor == nil {
		cfg.bgColor = transparent
	}
	if cfg.ori == orientationHorizontal && cfg.align == 0 {
		cfg.align = alignMiddle
	}

	el := &Element{
		// Container: *fyne.NewContainerWithLayout(lyt, objects...),
		cfg:  cfg,
		kids: kids,
	}
	el.lyt = &elementLayout{
		el: el,
	}
	el.Refresh()
	return el
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

	b.size = sz
	b.claimed = b.minSize
	cfg := b.cfg
	if cfg.position == positionAbsolute {
		b.claimed = sz
	} else if cfg.display == displayBlock {
		// block Element is greedy in x.
		b.claimed.Width = sz.Width
		if cfg.width != 0 {
			b.claimed.Width = cfg.width
		} else if b.claimed.Width < cfg.minW {
			b.claimed.Width = cfg.minW
		} else if cfg.maxW != 0 && b.claimed.Width > cfg.maxW {
			b.claimed.Width = cfg.maxW
		}
	}
	// Give the kids a chance to be greedy too.
	flowingKids, positionedKids := layoutItems(b.kids)
	for _, o := range flowingKids {
		if isNativeInline(o) {
			continue
		}
		o.Resize(b.claimed)
	}
	// Recalculate our own MinSize.
	b.Refresh()
	b.lyt.positionAbsolutely(positionedKids)
}

// Size returns the current size of this object.
func (b *Element) Size() fyne.Size {
	return b.claimed
}

// Refresh must be called if this object should be redrawn because its inner
// state changed.
func (b *Element) Refresh() {
	for _, o := range b.kids {
		o.Refresh()
	}

	cfg := b.cfg
	pt, pr, pb, pl := cfg.padding[0], cfg.padding[1], cfg.padding[2], cfg.padding[3]
	mt, mr, mb, ml := cfg.margins[0], cfg.margins[1], cfg.margins[2], cfg.margins[3]
	bw := cfg.borderWidth
	kidSize := b.lyt.MinSize(b.kids)
	minW := kidSize.Width + pl + pr + ml + mr + 2*bw
	minH := kidSize.Height + pt + pb + mt + mb + 2*bw

	b.minSize = fyne.NewSize(minW, minH)

	if cfg.position == positionAbsolute {
		return
	}

	if cfg.display == displayInline {
		b.claimed.Width = minW
	}

	b.claimed.Height = b.minSize.Height
	if cfg.height != 0 {
		b.claimed.Height = cfg.height
	} else if cfg.minH != 0 && b.claimed.Height < cfg.minH {
		b.claimed.Height = cfg.minH
	} else if cfg.maxH != 0 && b.claimed.Height > cfg.maxH {
		b.claimed.Height = cfg.maxH
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
	ears := b.cfg.listeners
	if ears.click != nil {
		ears.click(ev)
	}
}

func (b *Element) MouseIn(ev *desktop.MouseEvent) {
	ears := b.cfg.listeners
	if ears.mouseIn != nil {
		ears.mouseIn(ev)
	}
}

// MouseMoved is a hook that is called if the mouse pointer moved over the element.
func (b *Element) MouseMoved(ev *desktop.MouseEvent) {}

// MouseOut is a hook that is called if the mouse pointer leaves the element.
func (b *Element) MouseOut() {
	ears := b.cfg.listeners
	if ears.mouseOut != nil {
		ears.mouseOut()
	}
}

func (b *Element) Cursor() desktop.Cursor {
	return b.cfg.cursor
}

func (b *Element) setBackgroundColor(c color.Color) {
	b.cfg.bgColor = c
	if b.cachedRenderer != nil {
		b.cachedRenderer.Refresh()
		canvas.Refresh(b)
	}
}

type elementLayout struct {
	el *Element
}

func (lyt *elementLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	cfg := lyt.el.cfg
	if cfg.width != 0 && cfg.height != 0 {
		return fyne.NewSize(cfg.width, cfg.height)
	}
	minW, minH := cfg.minW, cfg.minH
	maxW, maxH := cfg.maxW, cfg.maxH
	w, h := lyt.actualMinSize(objects)

	if cfg.width != 0 {
		w = cfg.width
	} else {
		if minW != 0 && w < minW {
			w = minW
		}
		if maxW != 0 && w > maxW {
			w = maxW
		}
	}
	if cfg.height != 0 {
		h = cfg.height
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
	if lyt.el.cfg.ori == orientationVertical {
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
	cfg := lyt.el.cfg
	if cfg.ori == orientationHorizontal {
		lyt.layoutRow(objects, minW)
		return
	}

	lyt.layoutColumn(objects, minH)
}

func (lyt *elementLayout) minSizeRow(objects []fyne.CanvasObject) (w, h int) {
	cfg := lyt.el.cfg
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
		spacing = cfg.spacing
	}
	return w, h
}

func (lyt *elementLayout) minSizeColumn(objects []fyne.CanvasObject) (w, h int) {
	cfg := lyt.el.cfg
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
		spacing = cfg.spacing
	}
	return w, h
}

func (lyt *elementLayout) layoutRow(objects []fyne.CanvasObject, minW int) {
	claimed := lyt.el.claimed
	cfg := lyt.el.cfg
	justi, align := cfg.justi, cfg.align
	flowingObs, _ := layoutItems(objects)
	n := len(flowingObs)
	pr, pl := cfg.padding[1], cfg.padding[3]
	mr, ml := cfg.margins[1], cfg.margins[3]
	bw := cfg.borderWidth
	xSpace := 2*bw + ml + pl + mr + pr
	availWidth := claimed.Width - xSpace
	spacing := cfg.spacing

	var padding int
	if n > 1 && justi != justifyStart {
		divisor := (n - 1)
		if justi == justifyAround {
			divisor = n
		}
		padding = (availWidth - minW) / divisor
	}

	pos := fyne.NewPos(0, 0)
	if justi == justifyAround {
		pos = fyne.NewPos(padding/2, 0)
	}

	for _, o := range flowingObs {
		if !o.Visible() {
			continue
		}
		size := o.Size()

		var y int
		switch align {
		case alignMiddle:
			y = (claimed.Height - size.Height) / 2
		case alignBaseline:
			y = claimed.Height - size.Height
		}

		pos.Y = y

		o.Move(pos)

		// We're choosing center alignment here, but we could also implement
		// other alignments.
		pos.X += size.Width + padding + spacing
	}
}

func (lyt *elementLayout) layoutColumn(objects []fyne.CanvasObject, minH int) {

	claimed := lyt.el.claimed
	cfg := lyt.el.cfg
	justi, align := cfg.justi, cfg.align
	flowingObs, _ := layoutItems(objects)
	n := len(flowingObs)
	pt, pb := cfg.padding[0], cfg.padding[2]
	mt, mb := cfg.margins[0], cfg.margins[2]
	bw := cfg.borderWidth
	ySpace := 2*bw + mt + pt + mb + pb
	availHeight := claimed.Height - ySpace
	spacing := cfg.spacing

	var padding int
	if n > 1 && justi != justifyStart {
		divisor := (n - 1)
		if justi == justifyAround {
			divisor = n
		}
		padding = (availHeight - minH) / divisor
	}

	pos := fyne.NewPos(0, 0)
	if justi == justifyAround {
		pos = fyne.NewPos(padding/2, 0)
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
		case alignCenter:
			x = (claimed.Width - size.Width) / 2
		case alignRight:
			x = claimed.Width - size.Width
		}

		pos.X = x

		o.Move(pos)

		pos.Y += size.Height + padding + spacing
	}
}

func (lyt *elementLayout) positionAbsolutely(obs []fyne.CanvasObject) {
	if len(obs) == 0 {
		return
	}
	cfg, claimed := lyt.el.cfg, lyt.el.claimed
	mt, mr, mb, ml := cfg.margins[0], cfg.margins[1], cfg.margins[2], cfg.margins[3]
	bw := cfg.borderWidth
	absH := claimed.Height - mb - mt - 2*bw
	absW := claimed.Width - mr - ml - 2*bw

	for _, o := range obs {
		oCfg := o.(*Element).cfg // Already know this is an *Element because it was filtered through layoutItems.
		t, r, b, l := oCfg.top, oCfg.right, oCfg.bottom, oCfg.left
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
	return transparent
}

// Destroy is for internal use.
func (r *blockRenderer) Destroy() {}

// Layout is a hook that is called if the widget needs to be laid out.
// This should never call Refresh.
func (r *blockRenderer) Layout(sz fyne.Size) {
	cfg := r.el.cfg
	pt, pr, pb, pl := cfg.padding[0], cfg.padding[1], cfg.padding[2], cfg.padding[3]
	mt, mr, mb, ml := cfg.margins[0], cfg.margins[1], cfg.margins[2], cfg.margins[3]
	bw, br, brdc, bgc := cfg.borderWidth, cfg.borderRadius, cfg.borderColor, cfg.bgColor
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
	if cfg.borderWidth > 0 {
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

	if bgc != transparent {
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
	if r.el.name == "homeBox" {
		for _, o := range append(r.el.obs, r.el.kids...) {
			fmt.Printf("-- blockRenderer.Objects %T to %v, size %v \n", o, o.Position(), o.Size())
		}
	}

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
	return ok && el.cfg.position == positionAbsolute
}
