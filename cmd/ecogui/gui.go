package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/container"
	"fyne.io/fyne/driver/desktop"
	"fyne.io/fyne/widget"
	"github.com/buck54321/eco"
)

const (
	introductionText = "For the best security and the full range of Decred services, you'll want to sync the full blockchain, which will use around 5 GB of disk space. If you're only interested in basic wallet functionality, you may choose to sync in SPV mode, which will be very fast and use about 100 MB of disk space."
)

var (
	logoRsrc, decreditonBGPath, decreditonBGOnPath, dexLauncherBGPath,
	dexLaunchedBGPath, dcrctlLauncherBGPath, leftArrow, spinnerIcon,
	windowLogo, fontRegular, fontBold *fyne.StaticResource

	bttnColor    = stringToColor("#005")
	bgColor      = stringToColor("#000006")
	blockerColor = stringToColor("#000006aa")
	transparent  = stringToColor("#0000")
	white        = stringToColor("#fff")

	decredKeyBlue   = stringToColor("#2970ff")
	decredTurquoise = stringToColor("#2ed6a1")
	decredDarkBlue  = stringToColor("#091440")
	decredLightBlue = stringToColor("#70cbff")
	decredGreen     = stringToColor("#41bf53")
	decredOrange    = stringToColor("#ed6d47")

	defaultButtonColor      = stringToColor("#003")
	defaultButtonHoverColor = stringToColor("#005")
	buttonColor2            = stringToColor("#001a08")
	buttonHoverColor2       = stringToColor("#00251a")
	textColor               = stringToColor("#c1c1c1")
	cursorColor             = stringToColor("#2970fe")
	focusColor              = cursorColor
	black                   = stringToColor("#000")
	inputColor              = stringToColor("#111")
)

func init() {
	if _, err := os.Stat("static"); err == nil {
		staticRoot = "static"
	}
	logoRsrc = mustLoadStaticResource("eco-logo.png")
	decreditonBGPath = mustLoadStaticResource("decrediton-launcher.png")
	decreditonBGOnPath = mustLoadStaticResource("decrediton-launched.png")
	dexLauncherBGPath = mustLoadStaticResource("dex-launcher.png")
	dexLaunchedBGPath = mustLoadStaticResource("dex-launched.png")
	dcrctlLauncherBGPath = mustLoadStaticResource("dcrctl-plus.png")
	leftArrow = mustLoadStaticResource("larrow.svg")
	spinnerIcon = mustLoadStaticResource("spinner.png")
	windowLogo = mustLoadStaticResource("dcr-logo.png")
	fontRegular = mustLoadStaticResource("SourceSans3-Regular.ttf")
	fontBold = mustLoadStaticResource("source-sans-pro-semibold.ttf")
}

type GUI struct {
	ctx      context.Context
	app      fyne.App
	window   fyne.Window
	mainView *widget.ScrollContainer
	driver   fyne.Driver // gui.driver.StartAnimation(fyne.Animation)
	logo     *canvas.Image

	// Eco state data.
	stateMtx         sync.RWMutex
	ecoStatus        *eco.EcoState
	decreditonStatus *eco.ServiceStatus
	dexStatus        *eco.ServiceStatus

	// Intro page
	intro struct {
		box   *Element
		pw    *betterEntry
		pwRow *Element
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
		progress *ecoLabel
		appRow   *Element
	}

	// Apps
	decrediton struct {
		launcher *Element
		offImg   *canvas.Image
		onImg    *canvas.Image
	}
	dex struct {
		launcher   *Element
		offImg     *canvas.Image
		onImg      *canvas.Image
		spinnerBox *Element
		spinner    *spinner
	}
	dcrctl struct {
		// AppLauncher.
		launcher *Element
		// dcrctl+
		view       *Element
		results    *betterEntry
		input      *betterEntry
		spinnerBox *Element
		spinner    *spinner
	}
}

func NewGUI(ctx context.Context) *GUI {
	a := app.New()
	a.Settings().SetTheme(newDefaultTheme())
	w := a.NewWindow("Decred Eco")
	w.SetIcon(windowLogo)
	w.Resize(fyne.NewSize(1024, 768))

	mainView := container.NewVScroll(container.NewVBox(container.NewCenter()))

	gui := &GUI{
		ctx:      ctx,
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

func (gui *GUI) Run() {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var state *eco.MetaState
		for {
			var err error
			state, err = eco.State(ctx)
			if err == nil {
				break
			}
			gui.home.progress.setText("Unable to retrieve Eco state: %v", err)
			select {
			case <-time.After(time.Second * 5):
			case <-ctx.Done():
				return
			}
		}
		gui.storeEcoState(&state.Eco)

		fmt.Println("--eco state", dirtyEncode(state))

		st := state.Services["dcrd"]
		dcrdLoading := st == nil

		fmt.Println("--dcrLoading", dcrdLoading)

		if !dcrdLoading {
			gui.home.appRow.Show()
		}

		st = state.Services["dcrwallet"]
		walletSyncing := st == nil || st.Sync == nil || st.Sync.Progress < 0.999
		if walletSyncing {
			gui.dcrctl.spinnerBox.Show()
			gui.dcrctl.spinner.Show()
		}

		fmt.Println("--walletSyncing", walletSyncing)

		fmt.Println("--SyncMode full", state.Eco.SyncMode == eco.SyncModeFull)

		var dexLoading bool
		if state.Eco.SyncMode == eco.SyncModeFull {
			gui.dex.launcher.Show()
			if walletSyncing {
				dexLoading = true
				gui.dex.spinnerBox.Show()
				gui.dex.spinner.Show()
			}
		}

		if state.Eco.WalletExists {
			gui.intro.pwRow.Hide()
		}

		if state.Eco.SyncMode == eco.SyncModeUninitialized {
			gui.showIntroView()
		}

		gui.home.box.Refresh()
		canvas.Refresh(gui.home.appRow)

		eco.Feed(gui.ctx, &eco.EcoFeeders{
			SyncStatus: func(u *eco.Progress) {

				fmt.Println("--SyncStatus", dirtyEncode(u))

				if walletSyncing && u.Service == "dcrwallet" && u.Progress > 0.999 {
					walletSyncing = false
					gui.dcrctl.spinnerBox.Hide()
					gui.dcrctl.spinner.Hide()
					canvas.Refresh(gui.dcrctl.launcher)
					gui.dex.spinnerBox.Hide()
					gui.dex.spinner.Hide()
					canvas.Refresh(gui.dex.launcher)
				}
				if u.Service == "dcrd" {
					if dcrdLoading {
						dcrdLoading = false
						gui.home.appRow.Show()
						gui.home.box.Refresh()
						canvas.Refresh(gui.home.box)
					}
					gui.processDCRDSyncUpdate(u)
				}
			},
			ServiceStatus: func(st *eco.ServiceStatus) {

				fmt.Println("--ServiceStatus", dirtyEncode(st))

				if dexLoading && !walletSyncing {
					dexLoading = false
					if walletSyncing {
						gui.dex.spinnerBox.Show()
						gui.dex.spinner.Show()
						canvas.Refresh(gui.dex.launcher)
					}
				}

				switch st.Service {
				case "dexc":
					old := gui.storeDEXState(st)
					if old == nil || old.On != st.On {
						if st.On {
							gui.dex.onImg.Show()
							gui.dex.offImg.Hide()
						} else {
							gui.dex.onImg.Hide()
							gui.dex.offImg.Show()
						}
						canvas.Refresh(gui.dex.launcher)
					}
				case "decrediton":
					old := gui.storeDecreditonState(st)
					if old == nil || old.On != st.On {
						if st.On {
							gui.decrediton.onImg.Show()
							gui.decrediton.offImg.Hide()
						} else {
							gui.decrediton.onImg.Hide()
							gui.decrediton.offImg.Show()
						}
						canvas.Refresh(gui.decrediton.launcher)
					}
				}

			},
		})
		// gui.showHomeView()
	}()

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

	gui.intro.pwRow = newElement(&elementStyle{
		padding:      borderSpecs{10, 10, 10, 10},
		bgColor:      inputColor,
		borderRadius: 3,
		maxW:         450,
	}, pw)

	bttn1 := newEcoBttn(&bttnOpts{
		bgColor:    buttonColor2,
		hoverColor: buttonHoverColor2,
	}, "Full Sync", func(*fyne.PointEvent) {
		gui.initEco(eco.SyncModeFull)
	})

	bttn2 := newEcoBttn(nil, "Lite Mode (SPV)", func(*fyne.PointEvent) {
		gui.initEco(eco.SyncModeSPV)
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
		gui.intro.pwRow,
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
	gui.home.progress = newEcoLabel("syncing blockchain...", &textStyle{fontSize: 16})

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

	newSpinnerBox := func() (*Element, *spinner) {
		var zero int
		spinner := newSpinner(gui.ctx, 7, 40, decredKeyBlue, decredGreen)
		spinnerBox := newElement(&elementStyle{
			position: positionAbsolute,
			left:     &zero,
			right:    &zero,
			top:      &zero,
			bottom:   &zero,
			ori:      orientationHorizontal,
			align:    alignMiddle,
			justi:    justifyAround,
			bgColor:  blockerColor,
		},
			spinner,
		)
		spinnerBox.Hide()
		// spinner is hidden by default
		return spinnerBox, spinner
	}

	// Decrediton
	gui.decrediton.offImg = newSizedImage(decreditonBGPath, 0, 150)
	gui.decrediton.onImg = newSizedImage(decreditonBGOnPath, 0, 150)
	gui.decrediton.onImg.Hide()

	gui.decrediton.launcher = makeAppLauncher(func(*fyne.PointEvent) {
		st := gui.decreditonState()
		if st == nil {
			log.Errorf("Cannot start decrediton. Service not available.")
			return
		}
		if st.On {
			// Nothing to do
			return
		}
		eco.StartDecrediton(gui.ctx)
	},
		gui.decrediton.offImg,
		gui.decrediton.onImg,
	)

	// DEX
	gui.dex.offImg = newSizedImage(dexLauncherBGPath, 0, 150)
	gui.dex.onImg = newSizedImage(dexLaunchedBGPath, 0, 150)
	gui.dex.onImg.Hide()
	gui.dex.spinnerBox, gui.dex.spinner = newSpinnerBox()
	gui.dex.launcher = makeAppLauncher(func(*fyne.PointEvent) {
		st := gui.dexState()
		if st == nil {
			log.Errorf("Cannot start decrediton. Service not available.")
			return
		}
		if st.On {
			// Nothing to do
			return
		}
		eco.StartDEX(gui.ctx)
	},
		gui.dex.offImg,
		gui.dex.onImg,
		gui.dex.spinnerBox,
	)
	gui.dex.launcher.Hide()

	// DCRCtl+
	gui.dcrctl.spinnerBox, gui.dcrctl.spinner = newSpinnerBox()
	gui.dcrctl.launcher = makeAppLauncher(func(*fyne.PointEvent) {
		gui.showDCRCtl()
	},
		newSizedImage(dcrctlLauncherBGPath, 0, 150),
		gui.dcrctl.spinnerBox,
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
	gui.home.appRow.Hide()

	gui.home.appRow.name = "appRow"

	gui.home.box = newElement(
		&elementStyle{
			padding: borderSpecs{20, 0, 0, 0},
			align:   alignCenter,
			spacing: 20,
			// expandVertically: true,
			// justi:            justifyStart,
		},
		gui.logo,
		gui.home.progress,
		gui.home.appRow,
	)
	gui.home.box.name = "homeBox"
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
	oldState = gui.ecoStatus
	gui.ecoStatus = newState
	return oldState
}

func (gui *GUI) decreditonState() *eco.ServiceStatus {
	gui.stateMtx.RLock()
	defer gui.stateMtx.RUnlock()
	return gui.decreditonStatus
}

func (gui *GUI) storeDecreditonState(newState *eco.ServiceStatus) (oldState *eco.ServiceStatus) {
	gui.stateMtx.Lock()
	defer gui.stateMtx.Unlock()
	oldState = gui.decreditonStatus
	gui.decreditonStatus = newState
	return oldState
}

func (gui *GUI) storeDEXState(newState *eco.ServiceStatus) (oldState *eco.ServiceStatus) {
	gui.stateMtx.Lock()
	defer gui.stateMtx.Unlock()
	oldState = gui.dexStatus
	gui.dexStatus = newState
	return oldState
}

func (gui *GUI) dexState() *eco.ServiceStatus {
	gui.stateMtx.RLock()
	defer gui.stateMtx.RUnlock()
	return gui.dexStatus
}

func (gui *GUI) processDCRDSyncUpdate(u *eco.Progress) {
	if u.Err != "" {
		gui.home.progress.setText("dcrd sync error: %s", u.Err)
		return
	}
	gui.home.progress.setText("%s (%.0f%%)", u.Status, u.Progress*100)
	gui.home.progress.Refresh()
	gui.home.box.Refresh()
	canvas.Refresh(gui.home.box)
}

// initEco should be run in a goroutine.
func (gui *GUI) initEco(syncMode eco.SyncMode) {
	pw := gui.intro.pw.Text
	ch, err := eco.Init(gui.ctx, pw, syncMode)
	if err != nil {
		gui.download.msg.setText("Error initalizing Eco: %v", err)
		return
	}
	gui.showDownloadView()
	for {
		select {
		case u := <-ch:
			if err != nil {
				gui.download.msg.setText(err.Error())
				return
			}
			if u.Err != "" {
				gui.download.msg.setText(u.Err)
				return
			}
			gui.download.msg.setText(u.Status)
			gui.download.progress.setText("%.0f%%", u.Progress*100)
			gui.download.box.Refresh()
			canvas.Refresh(gui.download.box)
			if u.Progress > 0.9999 {
				if syncMode == eco.SyncModeFull {
					gui.dex.spinnerBox.Show()
					gui.dex.spinner.Show()
					gui.dex.launcher.Show()
				}
				gui.showHomeView()
				return
			}
		case <-gui.ctx.Done():
			return
		}
	}
}

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

func dirtyEncode(thing interface{}) string {
	b, _ := json.Marshal(thing)
	return string(b)
}
