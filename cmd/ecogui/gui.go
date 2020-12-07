package main

import (
	"context"
	"fmt"
	"image/color"
	"strings"
	"sync"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/container"
	"fyne.io/fyne/driver/desktop"
	"fyne.io/fyne/widget"
	"github.com/buck54321/eco"
)

var (
	decreditonBGPath     = mustLoadStaticResource("decrediton-launcher.png")
	decreditonBGOnPath   = mustLoadStaticResource("decrediton-launched.png")
	dexLauncherBGPath    = mustLoadStaticResource("dex-launcher.png")
	dexLaunchedBGPath    = mustLoadStaticResource("dex-launched.png")
	dcrctlLauncherBGPath = mustLoadStaticResource("dcrctl-plus.png")
	leftArrow            = mustLoadStaticResource("larrow.svg")
	spinnerIcon          = mustLoadStaticResource("spinner.png")
)

type GUI struct {
	ctx      context.Context
	app      fyne.App
	window   fyne.Window
	mainView *widget.ScrollContainer
	driver   fyne.Driver // gui.driver.StartAnimation(fyne.Animation)
	logo     *canvas.Image

	// Eco state data.
	stateMtx     sync.RWMutex
	ecoSt        *eco.EcoState
	decreditonSt *eco.ServiceStatus
	dexSt        *eco.ServiceStatus

	// Intro page
	intro struct {
		box *Element
		pw  *betterEntry
	}

	// Downloading page
	download struct {
		box      *Element
		progress *ecoLabel
		msg      *ecoLabel
	}

	// Home page
	home struct {
		box      *Element
		progress *widget.Label
		appRow   *Element
	}

	// Apps
	decrediton struct {
		launcher *Element
		offImg   *canvas.Image
		onImg    *canvas.Image
	}
	dex struct {
		launcher *Element
		offImg   *canvas.Image
		onImg    *canvas.Image
	}
	dcrctl struct {
		// AppLauncher.
		launcher *Element
		// dcrctl+
		view    *Element
		results *betterEntry
		input   *betterEntry
	}
}

func NewGUI() *GUI {
	a := app.New()
	a.Settings().SetTheme(newDefaultTheme())
	w := a.NewWindow("Decred Eco")
	w.SetIcon(windowLogo)
	w.Resize(fyne.NewSize(1024, 768))

	mainView := container.NewVScroll(container.NewVBox(container.NewCenter()))

	gui := &GUI{
		app:      a,
		driver:   a.Driver(),
		window:   w,
		mainView: mainView,
	}
	gui.logo = newSizedImage(logoRsrc, 0, 30)

	gui.initializeIntroView()
	gui.initializeDownloadView()
	gui.initializeHomeView()
	gui.initializeDCRCtl()

	gui.showHomeView()
	// gui.showDCRCtl()
	// gui.showDownloadView()
	// gui.showIntroView()
	// w.SetContent(container.NewVScroll(newPeeker()))

	return gui
}

func (gui *GUI) Run(ctx context.Context) {
	var wg sync.WaitGroup
	gui.ctx = ctx
	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	var err error
	// 	var state *eco.MetaState
	// 	for {
	// 		state, err = eco.State(ctx)
	// 		if err == nil {
	// 			break
	// 		}
	// 		log.Errorf("Unable to retrieve Eco state: %v. Trying again in 5 seconds", err)
	// 		select {
	// 		case <-time.After(time.Second * 5):
	// 		case <-ctx.Done():
	// 			return
	// 		}
	// 	}
	// 	gui.storeEcoState(&state.Eco)
	// 	if err != nil {
	// 		log.Errorf("Error retreiving Eco state: %v", err)
	// 		gui.processDCRDSyncUpdate(&eco.Progress{Err: "Error retreiving Eco state. Is the Eco system service running?"})
	// 		gui.showIntroView()
	// 		return
	// 	}
	// 	for svc, svcStatus := range state.Services {
	// 		switch svc {
	// 		case "decrediton":
	// 			gui.storeDecreditonStatus(svcStatus)
	// 		case "dexc":
	// 			gui.storeDEXStatus(svcStatus)
	// 		}
	// 	}
	// 	if state.Eco.WalletExists {
	// 		gui.introPW.SetText("")
	// 	}

	// 	// gui.updateApps()

	// 	if state.Eco.SyncMode == eco.SyncModeUninitialized {
	// 		gui.showIntroView()
	// 		return
	// 	}
	// 	gui.showHomeView()
	// }()

	// go func() {
	// 	ticker := time.NewTicker(time.Second)
	// 	for {
	// 		select {
	// 		case <-ctx.Done():
	// 			return
	// 		case <-ticker.C:
	// 			gui.download.progress.setText("%.1f%%", rand.Float32()*100)
	// 			gui.download.progress.Refresh()
	// 			gui.download.box.Refresh()
	// 			canvas.Refresh(gui.download.box)
	// 		}
	// 	}
	// }()

	gui.window.ShowAndRun()
	wg.Wait()
}

func (gui *GUI) setView(wgt fyne.CanvasObject) {
	gui.window.SetContent(container.NewVScroll(wgt))
}

func (gui *GUI) initializeIntroView() {
	intro := widget.NewLabel(introductionText)
	intro.Wrapping = fyne.TextWrapWord

	pw := &betterEntry{Entry: &widget.Entry{}, w: 430}
	gui.intro.pw = pw
	pw.PlaceHolder = "set your password"
	pw.Password = true
	pw.ExtendBaseWidget(pw)

	bttn1 := newEcoBttn(&bttnOpts{
		bgColor:    buttonColor2,
		hoverColor: buttonHoverColor2,
	}, "Full Sync", func(*fyne.PointEvent) {
		fmt.Println("Ooh. That felt nice.")
	})

	bttn2 := newEcoBttn(nil, "Lite Mode (SPV)", func(*fyne.PointEvent) {
		fmt.Println("That felt so so.")
	})

	bttnRow := newElement(&elementStyle{
		maxW:  450,
		justi: justifyAround,
		ori:   orientationHorizontal,
	},
		bttn1,
		bttn2,
	)

	gui.intro.box = newElement(
		&elementStyle{
			spacing: 30,
			padding: borderSpecs{20, 0, 0, 0},
			// maxW:    450,
			align: alignCenter,
			// bgColor: stringToColor("#ff0"),
		},
		gui.logo,
		newLabelWithWidth(intro, 430),
		newElement(&elementStyle{
			padding:      borderSpecs{10, 10, 10, 10},
			bgColor:      inputColor,
			borderRadius: 3,
			maxW:         450,
		}, pw),
		bttnRow,
	)
}

func (gui *GUI) showIntroView() {
	gui.setView(gui.intro.box)
}

func (gui *GUI) initializeDownloadView() {

	header := newEcoLabel("Downloading", &textStyle{
		fontSize: 18,
	})

	gui.download.progress = newEcoLabel("78.6%", &textStyle{
		fontSize: 40,
		bold:     true,
	})

	gui.download.msg = newEcoLabel("starting download...", nil)

	downloadView := newElement(
		&elementStyle{
			spacing: 20,
			padding: borderSpecs{20, 0, 20, 0},
			align:   alignCenter,
		},
		gui.logo,
		header,
		gui.download.progress,
		gui.download.msg,
	)
	downloadView.Refresh()

	gui.download.box = downloadView
}

func (gui *GUI) showDownloadView() {
	gui.setView(gui.download.box)
}

func (gui *GUI) initializeHomeView() {
	// A label describing the current sync state. Could do dcrd on left and
	// dcrwallet on right.
	gui.home.progress = widget.NewLabel("syncing blockchain...")
	gui.home.progress.Resize(gui.home.progress.MinSize())

	makeAppLauncher := func(click func(*fyne.PointEvent), imgs ...fyne.CanvasObject) *Element {
		return newElement(&elementStyle{
			padding: borderSpecs{5, 10, 5, 10},
			cursor:  desktop.PointerCursor,
			display: displayInline,
			listeners: eventListeners{
				click: click,
			},
		},
			imgs...,
		)
	}

	// Decrediton
	gui.decrediton.offImg = newSizedImage(decreditonBGPath, 0, 150)
	gui.decrediton.onImg = newSizedImage(decreditonBGOnPath, 0, 150)
	gui.decrediton.onImg.Hide()
	var state bool
	gui.decrediton.launcher = makeAppLauncher(func(*fyne.PointEvent) {
		if state {
			gui.decrediton.offImg.Show()
			gui.decrediton.onImg.Hide()
		} else {
			gui.decrediton.offImg.Hide()
			gui.decrediton.onImg.Show()
		}
		canvas.Refresh(gui.decrediton.launcher)
		state = !state
	},
		gui.decrediton.offImg,
		gui.decrediton.onImg,
	)

	// DEX
	gui.dex.offImg = newSizedImage(dexLauncherBGPath, 0, 150)
	gui.dex.onImg = newSizedImage(dexLaunchedBGPath, 0, 150)
	gui.dex.onImg.Hide()
	var dexState bool
	gui.dex.launcher = makeAppLauncher(func(*fyne.PointEvent) {
		if dexState {
			gui.dex.offImg.Show()
			gui.dex.onImg.Hide()
		} else {
			gui.dex.offImg.Hide()
			gui.dex.onImg.Show()
		}
		canvas.Refresh(gui.dex.launcher)
		dexState = !dexState
	},
		gui.dex.offImg,
		gui.dex.onImg,
	)

	// DCRCtl+
	gui.dcrctl.launcher = makeAppLauncher(func(*fyne.PointEvent) {
		gui.showDCRCtl()
	},
		newSizedImage(dcrctlLauncherBGPath, 0, 150),
	)

	// A horizontal div (with wrapping?) holding image buttons to start various
	// Decred services.
	gui.home.appRow = newElement(&elementStyle{
		ori:   orientationHorizontal,
		justi: justifyAround,
		align: alignCenter,
		maxW:  1000,
	},
		gui.decrediton.launcher,
		gui.dex.launcher,
		gui.dcrctl.launcher,
	)

	zero := 0
	absEl := newElement(&elementStyle{
		position: positionAbsolute,
		left:     &zero,
		top:      &zero,
		// bottom:   &zero,
		right:   &zero,
		width:   50,
		height:  50,
		bgColor: stringToColor("#0ff"),
	})

	gui.home.box = newElement(
		&elementStyle{
			padding: borderSpecs{80, 0, 0, 0},
			align:   alignCenter,
			spacing: 20,
			bgColor: stringToColor("#333"),
		},
		gui.logo,
		gui.home.progress,
		gui.home.appRow,
		absEl,
		newElement(&elementStyle{width: 50, height: 50, bgColor: stringToColor("#fee")}),
	)
}

func (gui *GUI) showHomeView() {
	gui.setView(gui.home.box)
}

func (gui *GUI) initializeDCRCtl() {
	larrow := canvas.NewImageFromResource(leftArrow)
	sz := fyne.NewSize(13, 13)
	larrow.Resize(sz)
	larrow.SetMinSize(sz)

	goHome := widget.NewLabel("back home")
	goHome.Resize(goHome.MinSize())

	var link *Element
	link = newElement(&elementStyle{
		padding: borderSpecs{-1, 5, -1, 5},
		bgColor: inputColor,
		cursor:  desktop.PointerCursor,
		ori:     orientationHorizontal,
		display: displayInline,
		spacing: 5,
		listeners: eventListeners{
			click: func(ev *fyne.PointEvent) {
				gui.showHomeView()
			},
			mouseIn: func(*desktop.MouseEvent) {
				link.setBackgroundColor(defaultButtonColor)
			},
			mouseOut: func() {
				link.setBackgroundColor(inputColor)
			},
		},
	}, larrow, goHome)

	linkRow := newElement(&elementStyle{
		align:   alignLeft,
		display: displayInline,
		minW:    750,
	},
		link,
	)

	var resultDiv *Element
	var results *betterEntry
	input := &betterEntry{Entry: &widget.Entry{}, w: 750}
	gui.dcrctl.input = input
	input.ExtendBaseWidget(input)
	input.PlaceHolder = "type your dcrctl command here"
	input.returnPressed = func() {
		resultDiv.Show()
		if input.Text == "" {
			return
		}
		res, err := eco.DCRCtl(gui.ctx, strings.TrimSpace(input.Text))
		if err == nil {
			gui.dcrctl.results.SetText(fmt.Sprintf("result for %q:\n%s", input.Text, res))
			input.SetText("")
		} else {
			gui.dcrctl.results.SetText(fmt.Sprintf("request error: %v", err))
		}

		results.Refresh()
		resultDiv.Refresh()
		gui.dcrctl.view.Refresh()

		canvas.Refresh(gui.dcrctl.view)
	}

	// TextStyle for monospace hopefully coming soon. https://github.com/fyne-io/fyne/pull/1630
	results = &betterEntry{Entry: &widget.Entry{ /* TextStyle: fyne.TextStyle{Monospace: true},*/ Text: ""}, w: 730, readOnly: true}
	gui.dcrctl.results = results
	results.ExtendBaseWidget(results)
	results.readOnly = true // Don't use the Entry.ReadOnly.
	results.MultiLine = true
	results.Wrapping = fyne.TextWrapWord
	results.textStyle = fyne.TextStyle{Monospace: true}

	resultDiv = newElement(&elementStyle{
		bgColor:      inputColor,
		padding:      borderSpecs{10, 10, 10, 10},
		margins:      borderSpecs{1, 1, 20, 1},
		borderRadius: 4,
		borderWidth:  1,
		borderColor:  stringToColor("#444"),
		display:      displayInline,
		minW:         730,
	},
		gui.dcrctl.results,
	)
	resultDiv.Hide()

	gui.dcrctl.view = newElement(
		&elementStyle{
			padding: borderSpecs{20, 0, 0, 0},
			align:   alignCenter,
			spacing: 15,
		},
		gui.logo,
		linkRow,
		newElement(&elementStyle{
			padding:      borderSpecs{10, 10, 10, 10},
			bgColor:      inputColor,
			borderRadius: 3,
			maxW:         750,
		}, input),
		resultDiv,
	)
}

func (gui *GUI) showDCRCtl() {
	gui.setView(gui.dcrctl.view)
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

func (gui *GUI) storeDEXStatus(newState *eco.ServiceStatus) (oldState *eco.ServiceStatus) {
	gui.stateMtx.Lock()
	defer gui.stateMtx.Unlock()
	oldState = gui.dexSt
	gui.dexSt = newState
	return oldState
}

func (gui *GUI) processDCRDSyncUpdate(u *eco.Progress) {
	if u.Err != "" {
		gui.home.progress.SetText("dcrd sync error: " + u.Err)
		return
	}
	gui.home.progress.SetText(fmt.Sprintf("%s (%.1f%%)", u.Status, u.Progress*100))
}

// func (gui *GUI) updateApps() {
// 	gui.appRow.DeleteChildren(false)
// 	walletReady := gui.decreditonStatus() != nil
// 	if walletReady {
// 		gui.appRow.AddChild(gui.apps.decrediton)
// 	}

// 	if gui.dexcStatus() != nil {
// 		gui.appRow.AddChild(gui.apps.dex)
// 	}

// 	if walletReady {
// 		gui.appRow.AddChild(gui.apps.dcrctl)
// 	}
// }

type peeker struct {
	min    fyne.Size
	size   fyne.Size
	pos    fyne.Position
	hidden bool
}

func newPeeker() *peeker {
	return &peeker{
		min:  fyne.NewSize(10, 10),
		size: fyne.NewSize(10, 10),
	}
}

func (p *peeker) MinSize() fyne.Size {
	fmt.Println("--peeker.MinSize")
	return p.min
}

// Move moves this object to the given position relative to its parent.
// This should only be called if your object is not in a container with a layout manager.
func (p *peeker) Move(pos fyne.Position) {
	fmt.Println("--peeker.Move", pos)
	p.pos = pos
}

// Position returns the current position of the object relative to its parent.
func (p *peeker) Position() fyne.Position {
	return p.pos
}

// Resize resizes this object to the given size.
// This should only be called if your object is not in a container with a layout manager.
func (p *peeker) Resize(sz fyne.Size) {
	fmt.Println("--peeker.Resize", sz)
	p.size = sz
}

// Size returns the current size of this object.
func (p *peeker) Size() fyne.Size {
	return p.size
}

// visibility

// Hide hides this object.
func (p *peeker) Hide() {
	p.hidden = true
}

// Visible returns whether this object is visible or not.
func (p *peeker) Visible() bool {
	return !p.hidden
}

// Show shows this object.
func (p *peeker) Show() {
	p.hidden = false
}

// Refresh must be called if this object should be redrawn because its inner state changed.
func (p *peeker) Refresh() {
	fmt.Println("--peeker.Refresh")
}

func (p *peeker) CreateRenderer() fyne.WidgetRenderer {
	return &peekerRenderer{p}
}

type peekerRenderer struct {
	peeker *peeker
}

func (r *peekerRenderer) BackgroundColor() color.Color {
	return transparent
}

// Destroy is for internal use.
func (r *peekerRenderer) Destroy() {}

// Layout is a hook that is called if the widget needs to be laid out.
// This should never call Refresh.
func (r *peekerRenderer) Layout(sz fyne.Size) {
	fmt.Println("--peekerRenderer.Layout", sz)
}

// MinSize returns the minimum size of the widget that is rendered by this renderer.
func (r *peekerRenderer) MinSize() fyne.Size {
	return r.peeker.MinSize()
}

// Objects returns all objects that should be drawn.
func (r *peekerRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{}
}

// Refresh is a hook that is called if the widget has updated and needs to be redrawn.
// This might trigger a Layout.
func (r *peekerRenderer) Refresh() {
	// fmt.Println("--peekerRenderer.Refresh")
}
