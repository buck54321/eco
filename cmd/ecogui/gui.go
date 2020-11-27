package main

import (
	"context"
	"fmt"
	"image"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"

	"github.com/buck54321/eco"
	"github.com/decred/slog"
	"github.com/goki/gi/gi"
	"github.com/goki/gi/gimain"
	"github.com/goki/gi/gist"
	"github.com/goki/gi/oswin"
	"github.com/goki/gi/oswin/mouse"
	"github.com/goki/gi/units"
	"github.com/goki/ki/ki"
	"github.com/goki/mat32"
)

const (
	bgColor               = "#000008"
	cardColor             = "#0b0b0b"
	buttonColor1          = "#000025"
	buttonColor2          = "#001a08"
	checkboxColor         = "#black"
	textColor             = "#e1e1e1"
	introductionText      = "For the best security and the full range of Decred services, you'll want to sync the full blockchain, which will use around 5 GB of disk space. If you're only interested in basic wallet functionality, you may choose to sync in SPV mode, which will be very fast and use about 100 MB of disk space."
	middleDot        rune = 0x2022
)

var (
	ctx, cancel        = context.WithCancel(context.Background())
	log                = slog.NewBackend(os.Stdout).Logger("GUI")
	logoPath           = filepath.Join("static", "eco-logo.png")
	decreditonBGPath   = filepath.Join("static", "decrediton-launcher.png")
	decreditonBGOnPath = filepath.Join("static", "decrediton-launched.png")
	dexLauncherBGPath  = filepath.Join("static", "dex-launcher.png")
	dexLaunchedBGPath  = filepath.Join("static", "dex-launched.png")
)

func main() {
	gimain.Main(run)
}

func run() {
	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Interrupt)
	go func() {
		<-killChan
		cancel()
	}()
	eco.UseLogger(slog.NewBackend(os.Stdout).Logger("ECO"))
	gui := NewGUI()
	gui.Run(ctx)
}

// GUI is the user interface.
type GUI struct {
	ctx         context.Context
	mainWin     *gi.Window
	mainFrame   *gi.Frame
	winViewport *gi.Viewport2D
	views       *Rotary

	// Eco state data.
	stateMtx     sync.RWMutex
	ecoSt        *eco.EcoState
	decreditonSt *eco.ServiceStatus

	// Eco initialization page
	start    *gi.Frame
	removePW func()

	// Initialization progress page
	init            *gi.Frame
	initProgress    *gi.Label
	initProgressLbl *gi.Label

	// Home page
	home     *gi.Frame
	homeSync *gi.Label
	apps     struct {
		decrediton *gi.Frame
		dex        *gi.Frame
	}
	setDecreditonImg func(bool)
}

// NewGUI creates a new *GUI. Call Run to open the window.
func NewGUI() *GUI {
	// These are only used the first time. The OS remembers the window size by
	// name after that or something.
	width := 1024
	height := 768

	mainWin := gi.NewMainWindow("decred-eco-window", "Decred Eco", width, height)
	// mainWin.BlurEvents = true
	restyle(mainWin)

	vp := mainWin.WinViewport2D()
	updt := vp.UpdateStart()

	mainFrame := mainWin.SetMainFrame()
	restyle(mainFrame, "padding: 20; spacing: 20; horizontal-align: center")
	mainFrame.SetStretchMaxWidth()

	_, logo, err := addImage(mainFrame, "mainFrame.logo", logoPath, 0, 40)
	if err != nil {
		log.Errorf("addImage error:", err)
	} else {
		logo.SetProp("horizontal-align", "center")
	}

	views := NewRotary(mainFrame, "sv", mat32.Y)

	gui := &GUI{
		mainWin:     mainWin,
		mainFrame:   mainFrame,
		winViewport: vp,
		views:       views,
	}
	gui.ecoSt = new(eco.EcoState)
	gui.decreditonSt = new(eco.ServiceStatus)

	gui.initializeStartBox()
	gui.initializeInitBox()
	gui.initializeHomeBox()
	gui.initializeSignals()

	gui.setMainView(gui.home)

	mainWin.SetCloseCleanFunc(func(w *gi.Window) {
		cancel()
		go gi.Quit() // once main window is closed, quit
	})

	vp.UpdateEndNoSig(updt)

	return gui
}

func (gui *GUI) setMainView(fr *gi.Frame) {
	updt := gui.mainFrame.UpdateStart()
	defer gui.mainFrame.UpdateEnd(updt)
	gui.views.setFrame(fr)
	gui.mainFrame.SetFullReRender()
}

// Run opens the GUI window.
func (gui *GUI) Run(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		gui.mainWin.StartEventLoop()
	}()
	gui.ctx = ctx
	// Run the startup routine, which will attempt to connect to the eco
	// eco service.
	wg.Add(1)
	go func() {
		defer wg.Done()
		state, err := eco.State(ctx)
		gui.storeEcoState(&state.Eco)
		if err != nil {
			log.Errorf("Error retreiving Eco state: %v", err)
			gui.signal(eventDCRDSyncStatus, &eco.Progress{Err: "Error retreiving Eco state. Is the Eco system service running?"})
			gui.sendSetFrameSignal(gui.start)
			return
		}
		if state.Eco.WalletExists {
			gui.removePW()
		}

		if state.Eco.SyncMode == eco.SyncModeUninitialized {
			gui.sendSetFrameSignal(gui.start)
			return
		}
		gui.sendSetFrameSignal(gui.home)
	}()

	wg.Add(1)
	eco.Feed(ctx, &eco.EcoFeeders{
		SyncStatus: func(u *eco.Progress) {
			if u.Service == "" && u.Err != "" {
				// internal error
				log.Errorf("Sync feed internal error: %s", u.Err)
				return
			}
			switch u.Service {
			case "dcrd":
				if u.Err == "" {
					gui.signal(eventDCRDSyncStatus, u)
				}
			default:
				log.Errorf("Sync update received for service with no update handler %q", u.Service)
			}
		},
		ServiceStatus: func(newState *eco.ServiceStatus) {
			switch newState.Service {
			case "decrediton":
				oldState := gui.storeDecreditonStatus(newState)
				if oldState.On != newState.On {
					gui.sendServiceStatusSignal(newState)
				}
			}
		},
	})

	wg.Wait()
}

func (gui *GUI) initializeSignals() {
	gui.mainWin.EventMgr.ConnectEvent(gui.mainWin, oswin.CustomEventType, gi.HiPri, func(recv, send ki.Ki, sig int64, d interface{}) {
		evt := d.(*oswin.CustomEvent).Data.(*customEvent)
		switch evt.eType {
		case eventSetFrame:
			frame := evt.data.(*gi.Frame)
			gui.setMainView(frame)
		case eventUpdateInitProgress:
			switch dt := evt.data.(type) {
			case *eco.Progress:
				gui.setInitProgress(dt, nil)
			case error:
				gui.setInitProgress(nil, dt)
			}
		case eventServiceStatus:
			u := evt.data.(*eco.ServiceStatus)
			switch u.Service {
			case "decrediton":
				gui.setDecreditonImg(u.On)
			}
		case eventDCRDSyncStatus:
			u := evt.data.(*eco.Progress)
			gui.processDCRDSyncUpdate(u)
		}
	})
}

func (gui *GUI) sendSetFrameSignal(frame *gi.Frame) {
	gui.signal(eventSetFrame, frame)
}

func (gui *GUI) sendServiceStatusSignal(u *eco.ServiceStatus) {
	gui.signal(eventServiceStatus, u)
}

func (gui *GUI) signal(evt int, thing interface{}) {
	gui.mainWin.SendCustomEvent(newCustomEvent(evt, thing))
}

func (gui *GUI) initializeStartBox() {
	var uhOh *gi.Label
	gui.start = gui.views.addNewFrame("gui.start", gi.LayoutVert)
	startBox := gi.AddNewFrame(gui.start, "startBox", gi.LayoutVert)
	restyle(startBox, ki.Props{
		"padding":        units.NewEm(0.6),
		"margin":         units.NewEm(0.1),
		"vertical-align": gist.AlignMiddle,
		"spacing":        30,
		// "border-width":     1,
		"border-color":     "#333",
		"horizontal-align": "center",
	}, false)

	intro := addNewLabel(startBox, "intro", introductionText, 15)
	restyle(intro, ki.Props{
		"max-width":        550,
		"white-space":      gist.WhiteSpacePreWrap,
		"word-wrap":        true,
		"text-align":       "center",
		"line-height":      1.25,
		"horizontal-align": "center",
	}, false)

	pwInput := &PasswordField{TextField: gi.AddNewTextField(startBox, "pwInput")}
	pwInput.NoEcho = true // https://github.com/goki/gi/pull/418
	pwInput.Placeholder = "set your password"
	pwInput.SetProp("clear-act", false)
	restyle(pwInput, ki.Props{
		"clear-act":        false, // No little clear button on righ side of input.
		"width":            300,
		"background-color": cardColor,
		"color":            textColor,
		"padding":          8,
		"margin":           1, // to accomodate border
		"horizontal-align": "center",
		"border-width":     0.5,
		"cursor-width":     2,
		gi.TextFieldSelectors[gi.TextFieldActive]: ki.Props{},
		gi.TextFieldSelectors[gi.TextFieldFocus]: ki.Props{
			"background-color": "#0f0f0f",
		},
		gi.TextFieldSelectors[gi.TextFieldInactive]: ki.Props{},
		gi.TextFieldSelectors[gi.TextFieldSel]:      ki.Props{},
	})

	gui.removePW = func() {
		updt := gui.start.UpdateStart()
		defer gui.start.UpdateEnd(updt)
		defer gui.start.SetFullReRender()
		startBox.DeleteChildByName("pwInput", false)
	}

	submitInit := func(syncMode eco.SyncMode) {
		updt := gui.views.UpdateStart()
		defer gui.views.UpdateEnd(updt)
		defer gui.views.ReRender2DTree()
		uhOh.SetText("")
		pw := pwInput.Txt
		if len(pw) == 0 && !gui.ecoState().WalletExists {
			uhOh.SetText("password cannot be empty")
			return
		}
		// We have a password and a sync mode. We can now begin the init
		// process. Switch to the status frame.
		gui.setMainView(gui.init)
		go gui.initEco(pw, syncMode)
	}

	bttnBox := gi.AddNewFrame(startBox, "bttnBox", gi.LayoutHoriz)
	restyle(bttnBox, "spacing: 10; horizontal-align: center")
	_, lbl := addNewButton(bttnBox, "bttnBox.opt1", "Full Sync", 0, buttonColor1, func(sig gi.ButtonSignals) {
		if sig == gi.ButtonClicked {
			submitInit(eco.SyncModeFull)
		}
	})
	lbl.SetProp("font-size", 14)
	lbl.SetProp("font-weight", gist.WeightSemiBold)

	addNewLabel(bttnBox, "bttnBox.or", "or", 20)

	_, lbl = addNewButton(bttnBox, "bttnBox.opt2", "Lite Mode (SPV)", 0, buttonColor2, func(sig gi.ButtonSignals) {
		if sig == gi.ButtonClicked {
			submitInit(eco.SyncModeSPV)
		}
	})
	lbl.SetProp("font-size", 14)
	lbl.SetProp("font-weight", gist.WeightSemiBold)

	uhOh = addNewLabel(startBox, "uhOh", "", 14)
	restyle(uhOh, "color: #a22; text-align: center;")
}

func (gui *GUI) ecoState() *eco.EcoState {
	gui.stateMtx.RLock()
	defer gui.stateMtx.RUnlock()
	return gui.ecoSt
}

func (gui *GUI) storeEcoState(newState *eco.EcoState) (oldState *eco.EcoState) {
	gui.stateMtx.Lock()
	defer gui.stateMtx.Unlock()
	oldState = gui.ecoSt
	gui.ecoSt = newState
	return oldState
}

func (gui *GUI) decreditonStatus() *eco.ServiceStatus {
	gui.stateMtx.RLock()
	defer gui.stateMtx.RUnlock()
	return gui.decreditonSt
}

func (gui *GUI) storeDecreditonStatus(newState *eco.ServiceStatus) (oldState *eco.ServiceStatus) {
	gui.stateMtx.Lock()
	defer gui.stateMtx.Unlock()
	oldState = gui.decreditonSt
	gui.decreditonSt = newState
	return oldState
}

// initEco should be run in a goroutine.
func (gui *GUI) initEco(pw string, syncMode eco.SyncMode) {
	ch, err := eco.Init(gui.ctx, pw, syncMode)
	if err != nil {
		gui.signal(eventUpdateInitProgress, fmt.Errorf("Error initalizing Eco: %w", err))
		return
	}
	for {
		select {
		case u := <-ch:
			gui.signal(eventUpdateInitProgress, u)
			if u.Progress > 0.99999 || u.Err != "" {
				return
			}
		case <-gui.ctx.Done():
			return
		}
	}
}

func (gui *GUI) setInitProgress(u *eco.Progress, err error) {
	updt := gui.init.UpdateStart()
	defer gui.init.UpdateEnd(updt)
	gui.init.SetFullReRender()
	if err != nil {
		gui.initProgressLbl.SetText(err.Error())
		return
	}
	if u.Err != "" {
		gui.initProgressLbl.SetText(u.Err)
		return
	}
	gui.initProgressLbl.SetText(u.Status)
	gui.initProgress.SetText(fmt.Sprintf("%.1f", u.Progress*100))
	if u.Progress > 0.9999 {
		gui.sendSetFrameSignal(gui.home)
	}
}

func (gui *GUI) initializeInitBox() {
	gui.init = gui.views.addNewFrame("gui.init", gi.LayoutVert)
	stateBox := gi.AddNewFrame(gui.init, "stateBox", gi.LayoutVert)
	restyle(stateBox, "horizontal-align: center; spacing: 15")
	progressRow := gi.AddNewFrame(stateBox, "progressRow", gi.LayoutHoriz)
	restyle(progressRow, "horizontal-align: center")
	gui.initProgress = addNewLabel(progressRow, "stateBox.initProgress", "50")
	gui.initProgress.SetProps(ki.Props{"font-size": 40, "vertical-align": "middle"}, false)
	pct := addNewLabel(progressRow, "pctLabel", "%")
	pct.SetProps(ki.Props{"font-size": 25, "vertical-align": "middle"}, false)
	gui.initProgressLbl = addNewLabel(stateBox, "stateBox.initProgressLbl", "This is the state view. It is the main view.")
	gui.initProgressLbl.SetProp("font-size", 14)
}

func (gui *GUI) initializeHomeBox() {
	gui.home = gui.views.addNewFrame("gui.home", gi.LayoutVert)
	homeBox := gi.AddNewFrame(gui.home, "homeBox", gi.LayoutVert)
	restyle(homeBox, "horizontal-align: center; spacing: 30;")
	gui.homeSync = addNewLabel(homeBox, "homeSync", "syncing...", 14)
	restyle(gui.homeSync, "horizontal-align: center;")

	row := gi.AddNewFrame(homeBox, "homeRow", gi.LayoutHoriz)
	restyle(row, "spacing: 50;")

	gi.AddNewStretch(row, "row.stretch.1")

	decreditonWgt := gi.AddNewFrame(row, "decrediton", gi.LayoutVert)
	gui.apps.decrediton = decreditonWgt
	restyle(decreditonWgt)
	decreditonOnImg, imgW, imgH, err := loadImage(decreditonBGOnPath, 350, 0)
	if err != nil {
		log.Errorf("loadImage error for %q: %w", decreditonBGOnPath, err)
	}
	decreditonOffImg, decreditonBitmap, err := addImage(decreditonWgt, "decrediton.bg", decreditonBGPath, 350, 0)
	if err == nil {
		decreditonBitmap.SetProp("horizontal-align", "center")
	} else {
		log.Errorf("Error adding decrediton background image: %v", err)
	}
	// This is a magic line that must be done to accept mouse events.
	decreditonWgt.Layout.Viewport = gui.winViewport
	bindClick(decreditonWgt, func() {
		if gui.decreditonStatus().On {
			// Nothing to do
			return
		}
		fmt.Println("--StartDecrediton")
		eco.StartDecrediton(gui.ctx)
	})

	gui.setDecreditonImg = func(on bool) {
		runWithUpdate(decreditonWgt, func() {
			if on {
				decreditonBitmap.SetImage(decreditonOnImg, float32(imgW), float32(imgH))
			} else {
				decreditonBitmap.SetImage(decreditonOffImg, float32(imgW), float32(imgH))
			}
		})
	}

	dexcWgt := gi.AddNewFrame(row, "dexc", gi.LayoutVert)
	gui.apps.dex = dexcWgt
	restyle(dexcWgt)

	dexOnImg, imgW, imgH, err := loadImage(dexLaunchedBGPath, 350, 0)
	if err != nil {
		log.Errorf("loadImage error for %q: %w", dexLaunchedBGPath, err)
	}
	dexOffImg, dexBitmap, err := addImage(dexcWgt, "dexc.launcher", dexLauncherBGPath, 350, 0)
	if err == nil {
		dexBitmap.SetProp("horizontal-align", "center")
	} else {
		log.Errorf("Error adding dex background image: %v", err)
	}
	dexcWgt.Layout.Viewport = gui.winViewport
	dexOn := false
	bindClick(dexcWgt, func() {
		if dexOn {
			dexBitmap.SetImage(dexOffImg, float32(imgW), float32(imgH))
			dexOn = false
		} else {
			dexBitmap.SetImage(dexOnImg, float32(imgW), float32(imgH))
			dexOn = true
		}
	})

	gi.AddNewStretch(row, "row.stretch.2")
}

func (gui *GUI) processDCRDSyncUpdate(u *eco.Progress) {
	updt := gui.home.UpdateStart()
	defer gui.home.UpdateEnd(updt)
	defer gui.home.SetFullReRender()
	if u.Err != "" {
		gui.homeSync.SetText("dcrd sync error: " + u.Err)
		return
	}
	gui.homeSync.SetText(fmt.Sprintf("%s (%.1f%%)", u.Status, u.Progress*100))
}

var cssReset = ki.Props{
	"background-color": bgColor,
	"margin":           0.0,
	"padding":          0.0,
	"border-style":     "none",
	"border-width":     0,
	"border-radius":    0,
}

func restyle(n ki.Ki, extraPropses ...interface{}) {
	n.SetProps(cssReset, false)
	if len(extraPropses) == 0 {
		return
	}
	extraProps := extraPropses[0]
	switch propsT := extraProps.(type) {
	case string:
		// style attribute-style string
		decs := strings.Split(strings.Trim(propsT, " ;"), ";")
		if len(decs) > 0 {
			props := make(ki.Props, len(decs))
			for _, dec := range decs {
				kv := strings.Split(dec, ":")
				if len(kv) != 2 {
					log.Warnf("invalid css declaration:", dec)
					continue
				}
				props[strings.Trim(kv[0], " ")] = strings.Trim(kv[1], " ")
			}
			if len(props) > 0 {
				n.SetProps(props, false)
			}
		}
	case ki.Props:
		if len(propsT) > 0 {
			n.SetProps(propsT, false)
		}
	}
}

func addNewButton(parent ki.Ki, name, txt string, wrapWidth int, color string, clickFunc func(gi.ButtonSignals)) (*gi.Button, *gi.Label) {
	bttn := gi.AddNewButton(parent, "bttn")
	bttn.SetText(txt) // Calls ConfigParts internally
	restyle(&bttn.Parts, ki.Props{"background-color": color})
	restyle(bttn, ki.Props{
		"padding":          units.NewEm(0.5),
		"background-color": color,
		"border-color":     "#666",
		"border-width":     units.NewPx(0.5),
		"margin":           units.NewPx(1), // margins must accomodate borders.
		"border-radius":    units.NewPx(2),
		// I feel like I shouldn't need to set the color, since I'm setting it
		// on the Label, but a full window re-render doesn't seem to pick up on
		// the Label's color, but does seem to propagate this one.
		"color":                               textColor,
		gi.ButtonSelectors[gi.ButtonActive]:   ki.Props{},
		gi.ButtonSelectors[gi.ButtonInactive]: ki.Props{},
		// gi.ButtonSelectors[gi.ButtonHover]:    ki.Props{},
		gi.ButtonSelectors[gi.ButtonFocus]:    ki.Props{},
		gi.ButtonSelectors[gi.ButtonDown]:     ki.Props{},
		gi.ButtonSelectors[gi.ButtonSelected]: ki.Props{},
	})

	lbl := bttn.Parts.ChildByName("label", 0)
	restyle(lbl, ki.Props{
		"color":            textColor,
		"font-size":        units.NewPx(13),
		"background-color": color,
	}, false)
	if clickFunc != nil {
		bttn.ButtonSig.Connect(bttn.This(), func(recv, send ki.Ki, sig int64, data interface{}) {
			clickFunc(gi.ButtonSignals(sig))
		})
	}
	if wrapWidth != 0 {
		bttn.SetProp("max-width", wrapWidth)
		lbl.SetProps(ki.Props{
			"max-width":   wrapWidth,
			"white-space": gist.WhiteSpacePreWrap,
			"word-wrap":   true,
		}, false)
	}
	return bttn, lbl.(*gi.Label)
}

func addNewCheckBox(parent ki.Ki, name string, toggleFunc func(newState bool)) *gi.CheckBox {
	cb := gi.AddNewCheckBox(parent, "cb")
	cb.ConfigParts()
	css := fmt.Sprintf("background-color: %s", checkboxColor)
	restyle(cb, css)
	restyle(&cb.Parts, css)
	stack := cb.Parts.Child(0)
	restyle(stack, css)
	// cb.Parts.SetPropChildren("color", textColor)
	stack.SetPropChildren("fill", checkboxColor)
	stack.SetPropChildren("stroke", textColor)
	if toggleFunc != nil {
		cb.ButtonSig.Connect(cb.This(), func(recv, send ki.Ki, sig int64, data interface{}) {
			if gi.ButtonSignals(sig) == gi.ButtonToggled {
				toggleFunc(cb.IsChecked())
			}
		})
	}
	return cb
}

func addImage(parent ki.Ki, name, imgPath string, w, h int) (image.Image, *gi.Bitmap, error) {
	img, w, h, err := loadImage(imgPath, w, h)
	if err != nil {
		return nil, nil, err
	}

	bm := gi.AddNewBitmap(parent, "dcrLogo")
	bm.SetProps(ki.Props{"width": w, "height": h}, false)
	bm.SetImage(img, float32(w), float32(h))
	return img, bm, nil
}

func loadImage(imgPath string, w, h int) (image.Image, int, int, error) {
	img, err := gi.OpenPNG(imgPath)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("OpenPNG error: %w", err)
	}

	bounds := img.Bounds()
	imgW := bounds.Max.X - bounds.Min.X
	imgH := bounds.Max.Y - bounds.Min.Y
	aspect := float64(imgW) / float64(imgH)
	if w == 0 {
		if h == 0 {
			w = imgW
			h = imgH
		} else {
			w = int(math.Round(aspect * float64(h)))
		}
	} else if h == 0 {
		h = int(math.Round(float64(w) / aspect))
	}
	return img, w, h, nil
}

func addNewLabel(parent ki.Ki, name, txt string, fontSize ...interface{}) *gi.Label {
	lbl := gi.AddNewLabel(parent, name, txt)
	lbl.SetProp("color", textColor)
	if len(fontSize) == 0 {
		fontSize = []interface{}{12}
	}
	lbl.SetProp("font-size", fontSize[0])
	return lbl
}

func addNewSpace(parent ki.Ki, name string, w interface{}) {
	gi.AddNewSpace(parent, name).SetProp("width", w)
}

type Rotary struct {
	*gi.SplitView
	parent *gi.Frame
	frames []*gi.Frame
}

func NewRotary(parent *gi.Frame, name string, dim mat32.Dims) *Rotary {
	// I would have liked to use a regular Frame with a LayoutStacked, but the
	// height of all frames is set to the height of the tallest, which has
	// weird side effects.
	sv := gi.AddNewSplitView(parent, name)
	sv.Dim = dim
	restyle(sv)
	return &Rotary{SplitView: sv, parent: parent}
}

func (r *Rotary) addNewFrame(name string, lyt gi.Layouts) *gi.Frame {
	frame := gi.AddNewFrame(r, name, lyt)
	restyle(frame)
	r.frames = append(r.frames, frame)
	r.UpdateSplits()
	return frame
}

func (r *Rotary) setFrame(frame *gi.Frame) {
	// updt := r.parent.UpdateStart()
	// defer r.parent.UpdateEnd(updt)
	splits := make([]float32, 0, len(r.frames))
	for _, fr := range r.frames {
		if frame == fr {
			splits = append(splits, 1)
		} else {
			splits = append(splits, 0)
		}
	}
	r.SetSplits(splits...)
	// r.parent.ReRender2DTree()
}

type PasswordField struct {
	*gi.TextField
}

type eventConnector interface {
	ki.Ki
	ConnectEvent(oswin.EventType, gi.EventPris, ki.RecvFunc)
	SetFullReRender()
}

func bindClick(wgt eventConnector, f func()) {
	wgt.ConnectEvent(oswin.MouseEvent, gi.RegPri, func(_, _ ki.Ki, _ int64, ev interface{}) {
		me := ev.(*mouse.Event)
		if me.Action == mouse.Press && me.Button == mouse.Left {
			updt := wgt.UpdateStart()
			defer wgt.UpdateEnd(updt)
			defer wgt.SetFullReRender()
			f()
		}
	})
}

func runWithUpdate(wgt eventConnector, f func()) {
	updt := wgt.UpdateStart()
	defer wgt.UpdateEnd(updt)
	defer wgt.SetFullReRender()
	f()
}