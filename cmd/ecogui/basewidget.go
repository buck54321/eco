package main

import "fyne.io/fyne"

type baseWidget struct {
	hidden bool
	pos    fyne.Position
}

func (w *baseWidget) Move(pos fyne.Position) {
	w.pos = pos
}

// Position returns the current position of the object relative to its parent.
func (w *baseWidget) Position() fyne.Position {
	return w.pos
}

// Hide hides this object.
func (w *baseWidget) Hide() {
	w.hidden = true
}

// Visible returns whether this object is visible or not.
func (w *baseWidget) Visible() bool {
	return !w.hidden
}

// Show shows this object.
func (w *baseWidget) Show() {
	w.hidden = false
}
