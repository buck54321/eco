package main

import (
	"context"
	"image/color"
	"math"
	"sync"
	"time"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"github.com/buck54321/eco/ui"
)

type spinner struct {
	*fyne.Container
	mtx    sync.Mutex
	cancel context.CancelFunc
	ctx    context.Context
	min    fyne.Size
	size   fyne.Size
	pos    fyne.Position
	hidden bool
	// Spinner fields.
	w    int
	n    int
	dots []fyne.CanvasObject // *canvas.Circle
}

// newSpinner is the constructor for a spinner. The spinner is not visible until
// Show is called. The spinner will be constructed with n dots, and will always
// be rendered at size w x w. The default color is white, but if colors are
// supplied, they will be applied repeating-sequentially to the dots.
func newSpinner(ctx context.Context, n, w int, colors ...color.Color) *spinner {
	if len(colors) == 0 {
		colors = []color.Color{ui.White}
	}
	nCol := len(colors)
	dots := make([]fyne.CanvasObject, 0, n)
	for i := 0; i < n; i++ {
		circ := canvas.NewCircle(colors[i%nCol])
		dots = append(dots, circ)
	}
	return &spinner{
		ctx:    ctx,
		min:    fyne.NewSize(w, w),
		size:   fyne.NewSize(w, w),
		w:      w,
		n:      n,
		dots:   dots,
		hidden: true,
	}
}

// MinSize for the spinner is always w x w.
func (s *spinner) MinSize() fyne.Size {
	return s.min
}

// Move moves this object to the given position relative to its parent. This
// should only be called if your object is not in a container with a layout
// manager.
func (s *spinner) Move(pos fyne.Position) {
	s.pos = pos
}

// Position returns the current position of the object relative to its parent.
func (s *spinner) Position() fyne.Position {
	return s.pos
}

// Resize resizes this object to the given size. This should only be called if
// your object is not in a container with a layout manager.
func (s *spinner) Resize(sz fyne.Size) {
	s.size = sz
}

// Size returns the current size of this object.
func (s *spinner) Size() fyne.Size {
	return s.min
	// return s.size
}

// Hide cancels any running animation and hides the spinner.
func (s *spinner) Hide() {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.hidden = true
}

// Visible returns whether this object is visible or not.
func (s *spinner) Visible() bool {
	return !s.hidden
}

// Show sets the hidden flag and starts an animation loop.
func (s *spinner) Show() {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
	var ctx context.Context
	ctx, s.cancel = context.WithCancel(s.ctx)
	go func() {
		ticker := time.NewTicker(time.Millisecond * 33) // ~ 30 fps
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.Refresh()
				canvas.Refresh(s)
			}
		}
	}()

	s.hidden = false
}

// Refresh sets the size and position of the dots.
func (s *spinner) Refresh() {
	increment := math.Pi * 2 / float64(s.n)
	halfW := float64(s.w) / 2
	rDots := halfW * 0.65
	r0 := rDots * math.Sin(increment/2) * 0.90
	rps := 1.0
	rotator := float64(time.Now().UnixNano()) / 1e9 * rps
	phi := (rotator - math.Floor(rotator)) * 2 * math.Pi

	for i, dot := range s.dots {
		angle := float64(i) * increment
		cx := round(math.Cos(angle)*rDots + halfW)
		cy := round(math.Sin(angle)*rDots + halfW)
		r := r0 - ((math.Cos(angle-phi) + 1) / 2 * r0 * 0.80)
		// We must apply the rounding to the radius and take the diameter as
		// twice ri, because the intregral diameter must be a multiple of two in
		// order to keep the dot center fixed, preventing the dots from
		// appearing to jump around as they change size.
		ri := round(r)
		di := 2 * ri
		dot.Resize(fyne.NewSize(di, di))

		dot.Move(fyne.NewPos(cx-ri, cy-ri))
	}
}

// CreateRenderer creates the spinnerRenderer. Part of the fyne.Widget
// interface.
func (s *spinner) CreateRenderer() fyne.WidgetRenderer {
	return &spinnerRenderer{s}
}

// spinnerRenderer is the fyne.WidgetRenderer for the spinner.
type spinnerRenderer struct {
	s *spinner
}

// BackgroundColor for the spinner is transparent.
func (r *spinnerRenderer) BackgroundColor() color.Color {
	return ui.Transparent
}

// Destroy is for fyne internal use.
func (r *spinnerRenderer) Destroy() {}

// Layout is a hook that is called if the widget needs to be laid out.
// This should never call Refresh.
func (r *spinnerRenderer) Layout(sz fyne.Size) {}

// MinSize returns the minimum size of the widget that is rendered by this
// renderer.
func (r *spinnerRenderer) MinSize() fyne.Size {
	return r.s.MinSize()
}

// Objects returns all objects that should be drawn.
func (r *spinnerRenderer) Objects() []fyne.CanvasObject {
	return r.s.dots
}

// Refresh is a hook that is called if the widget has updated and needs to be
// redrawn. This might trigger a Layout.
func (r *spinnerRenderer) Refresh() {
	r.s.Refresh()
}

func round(v float64) int {
	return int(math.Round(v))
}
