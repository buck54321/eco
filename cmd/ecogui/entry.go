package main

import (
	"image/color"
	"sync"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/widget"
)

type betterEntryRenderer struct {
	fyne.WidgetRenderer
	baseObjects []fyne.CanvasObject
	be          *betterEntry
}

func (r *betterEntryRenderer) Objects() []fyne.CanvasObject {
	objects := r.WidgetRenderer.Objects()
	pass := r.be.Password

	firstRect := func(obs []fyne.CanvasObject) (i int, found bool) {
		for i = range obs {
			if _, ok := obs[i].(*canvas.Rectangle); ok {
				found = true
				return
			}
		}
		return
	}

	// If this is our first pass through, and things look as expected, remove a
	// couple of unwanted style elements.
	if len(r.baseObjects) == 0 && ((len(objects) == 4 || !pass) || (len(objects) == 5 && pass)) { // Password fields have and extra element at the end, making 5
		obs := make([]fyne.CanvasObject, len(objects))
		copy(obs, objects)

		removeBaseObject := func(i int) {
			o := make([]fyne.CanvasObject, len(obs)-1)
			copy(o, obs[:i])
			copy(o[i:], obs[i+1:])
			obs = o
		}

		i, found := firstRect(objects)
		if found {
			removeBaseObject(i)
		}

		// The next rectangle is the cursor. Set the fill color, or remove if
		// readOnly.
		i, found = firstRect(obs)
		if found {
			cursor, ok := obs[i].(*canvas.Rectangle)
			if ok {
				if r.be.readOnly {
					removeBaseObject(i)
				} else {
					cursor.FillColor = cursorColor
					sz := cursor.Size()
					cursor.Resize(fyne.NewSize(sz.Width, sz.Height-(&defaultTheme{}).Padding()))
				}
			}
		}
		r.baseObjects = obs
	}

	// Grab the selections, but the last last rect is the underline again.
	selections := make([]fyne.CanvasObject, 0)
	if len(r.baseObjects) > 0 && ((len(objects) > 4 && !pass) || (len(objects) > 5 && pass)) {
		// pop selections
		for _, o := range objects {
			if _, ok := o.(*canvas.Rectangle); !ok {
				break
			}
			selections = append(selections, o)
		}
	}
	if len(selections) > 0 { // Remove underline
		selections = selections[:len(selections)-1]
	}

	return append(selections, r.baseObjects...)
}

func (r *betterEntryRenderer) BackgroundColor() color.Color {
	return transparent
}

type betterEntry struct {
	// The (BaseWidget).properyLock handling in (*Entry).TypedKey is pretty janky, and has
	// caused some crashes. We'll use our own.
	mtx sync.Mutex
	*widget.Entry
	w              int
	cachedRenderer fyne.WidgetRenderer
	returnPressed  func()
	readOnly       bool
	textStyle      fyne.TextStyle
}

func (be *betterEntry) CreateRenderer() fyne.WidgetRenderer {
	return &betterEntryRenderer{WidgetRenderer: be.Entry.CreateRenderer(), be: be}
}

func (be *betterEntry) MinSize() fyne.Size {
	baseSz := be.Entry.MinSize()
	baseSz.Width = be.w
	return baseSz
	// if be.h == 0 {
	// 	be.h = baseSz.Height - 3
	// }
	// baseSz.Height = be.h
	// return baseSz
}

func (be *betterEntry) Refresh() {
	// be.h = 0
	be.Entry.Refresh()
	be.Resize(be.Entry.MinSize())
}

func (be *betterEntry) Resize(sz fyne.Size) {
	// betterEntry has a fixed width for now.
	be.Entry.Resize(fyne.NewSize(be.w, sz.Height))
}

func (be *betterEntry) SetText(txt string) {
	be.mtx.Lock()
	defer be.mtx.Unlock()
	be.Entry.SetText(txt)
	// be.h = 0
	be.MinSize()
}

func (be *betterEntry) KeyDown(ev *fyne.KeyEvent) {
	if be.readOnly {
		return
	}
	be.Entry.KeyDown(ev)
	if (ev.Name == fyne.KeyEnter || ev.Name == fyne.KeyReturn) && be.returnPressed != nil {
		be.returnPressed()
	}
}

func (be *betterEntry) KeyUp(ev *fyne.KeyEvent) {
	if be.readOnly {
		return
	}
	be.Entry.KeyDown(ev)
}

func (be *betterEntry) TypedKey(key *fyne.KeyEvent) {
	if be.readOnly {
		return
	}
	be.mtx.Lock()
	defer be.mtx.Unlock()
	be.Entry.TypedKey(key)
}

func (be *betterEntry) TypedRune(key rune) {
	if be.readOnly {
		return
	}
	be.mtx.Lock()
	defer be.mtx.Unlock()
	be.Entry.TypedRune(key)
}

// FocusGained is a hook called by the focus handling logic after this object gained the focus.
func (be *betterEntry) FocusGained() {
	be.Entry.FocusGained()
}

// FocusLost is a hook called by the focus handling logic after this object lost the focus.
func (be *betterEntry) FocusLost() {
	be.Entry.FocusLost()
}

// Deprecated: this is an internal detail, canvas tracks current focused object
func (be *betterEntry) Focused() bool {
	return be.Entry.Focused()
}

func (be *betterEntry) TypedShortcut(shortcut fyne.Shortcut) {
	if be.readOnly && shortcut.ShortcutName() != "Copy" {
		return
	}
	be.mtx.Lock()
	defer be.mtx.Unlock()
	be.Entry.TypedShortcut(shortcut)
}
