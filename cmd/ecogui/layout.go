package main

import (
	"fyne.io/fyne"
)

type orientation uint8

const (
	orientationVertical orientation = iota
	orientationHorizontal
)

type alignment uint8

const (
	// Horizontal alignments. left typically chosen when outside of valid range.
	alignLeft alignment = iota
	alignCenter
	alignRight
	// Vertical alignments. middle typically chosen when outside of valid range.
	alignTop
	alignMiddle
	alignBaseline
)

// Used for padding and margins, but could also be used for border widths,
// border radii, etc.
type borderSpecs [4]int

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
	justifyStart
)

// type flexOpts struct {
// 	w, h          int
// 	justification flexJustification
// }

// type flexRow struct {
// 	opts *flexOpts
// }

// func newFlexRow(opts *flexOpts, objects ...fyne.CanvasObject) *fyne.Container {
// 	return fyne.NewContainerWithLayout(&flexRow{
// 		opts: opts,
// 	}, objects...)
// }

// func (lyt *flexRow) MinSize(objects []fyne.CanvasObject) fyne.Size {
// 	w, h := lyt.actualMinSize(objects)
// 	if lyt.opts.w != 0 {
// 		w = lyt.opts.w
// 	}
// 	if lyt.opts.h != 0 {
// 		h = lyt.opts.h
// 	}
// 	return fyne.NewSize(w, h)
// }

// func (lyt *flexRow) actualMinSize(objects []fyne.CanvasObject) (w, h int) {
// 	return minSizeRow(objects)
// }

// func (lyt *flexRow) Layout(objects []fyne.CanvasObject, containerSize fyne.Size) {
// 	n := len(objects)
// 	if n == 0 {
// 		return
// 	}

// 	minW, _ := lyt.actualMinSize(objects)

// 	j := lyt.opts.justification

// 	var padding int
// 	if n > 1 {
// 		divisor := (n - 1)
// 		if j == justifyAround {
// 			divisor = n
// 		}
// 		padding = (containerSize.Width - minW) / divisor
// 	}

// 	pos := fyne.NewPos(0, 0)
// 	if j == justifyAround {
// 		pos = fyne.NewPos(padding/2, 0)
// 	}

// 	for _, o := range objects {
// 		size := o.MinSize()
// 		o.Resize(size)
// 		o.Move(pos)
// 		pos = pos.Add(fyne.NewPos(size.Width+padding, 0))
// 	}
// }

func layoutItems(objects []fyne.CanvasObject) (flowing []fyne.CanvasObject, positioned []fyne.CanvasObject) {
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		if isAbsolutelyPositioned(o) {
			positioned = append(positioned, o)
			continue
		}
		flowing = append(flowing, o)
	}
	return
}
