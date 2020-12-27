package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/driver/desktop"
	"fyne.io/fyne/widget"
	"github.com/buck54321/eco"
	"github.com/buck54321/eco/ui"
)

const (
	introductionText = "For the best security and the full range of Decred services, you'll want to sync the full blockchain, which will use around 5 GB of disk space. If you're only interested in basic wallet functionality, you may choose to sync in SPV mode, which will be very fast and use about 100 MB of disk space."
)

var (
	ecoLogo, decreditonLogo, dexLogo, dcrctlLogo, leftArrow, windowLogo,
	fontRegular, fontBold *fyne.StaticResource

	staticRoot = filepath.Join(eco.EcoDir, "static")
)

func init() {
	// If we're running from the repo cmd/ecogui directory, use the repo static
	if _, err := os.Stat("static"); err == nil {
		staticRoot = "static"
	}
	ecoLogo = loadStatic("eco-logo.png")
	decreditonLogo = loadStatic("decrediton.png")
	dexLogo = loadStatic("dcrdex.png")
	dcrctlLogo = loadStatic("dcrctl-plus.png")

	// decreditonBGPath = loadStatic("decrediton-launcher.png")
	// decreditonBGOnPath = loadStatic("decrediton-launched.png")
	// dexLauncherBGPath = loadStatic("dex-launcher.png")
	// dexLaunchedBGPath = loadStatic("dex-launched.png")
	// dcrctlLauncherBGPath = loadStatic("dcrctl-plus.png")

	leftArrow = loadStatic("larrow.svg")
	windowLogo = loadStatic("dcr-logo.png")
	// fontRegular = loadStatic("SourceSans3-Regular.ttf")
	// fontBold = loadStatic("source-sans-pro-semibold.ttf")
}

func loadStatic(rsc string) *fyne.StaticResource {
	return ui.MustLoadStaticResource(filepath.Join(staticRoot, rsc))
}

type GUI struct {
	ctx      context.Context
	quit     context.CancelFunc
	app      fyne.App
	window   fyne.Window
	driver   fyne.Driver // gui.driver.StartAnimation(fyne.Animation)
	logo     *canvas.Image
	mainView *ui.Element

	// Eco state data.
	stateMtx         sync.RWMutex
	ecoStatus        *eco.EcoState
	decreditonStatus *eco.ServiceStatus
	dexStatus        *eco.ServiceStatus

	// Intro page
	intro struct {
		box   *ui.Element
		pw    *betterEntry
		pwRow *ui.Element
	}

	// Downloading page
	download struct {
		box      *ui.Element
		progress *ui.EcoLabel
		msg      *ui.EcoLabel
	}

	// Home page
	home struct {
		box          *ui.Element
		dcrdProgress *ui.EcoLabel
		dcrwProgress *ui.EcoLabel
		appRow       *ui.Element
		xcRate       *ui.EcoLabel
		xcDatum      *ui.Element
		stakeDiff    *ui.EcoLabel
		sdDatum      *ui.Element
		hashRate     *ui.EcoLabel
		hrDatum      *ui.Element
		blockHeight  *ui.EcoLabel
		bhDatum      *ui.Element
		stats        *ui.Element
	}

	// Apps
	decrediton struct {
		launcher *ui.Element
	}

	dex struct {
		launcher   *ui.Element
		spinnerBox *ui.Element
		spinner    *spinner
	}

	dcrctl struct {
		// AppLauncher.
		launcher *ui.Element
		// dcrctl+
		view       *ui.Element
		results    *betterEntry
		input      *betterEntry
		spinnerBox *ui.Element
		spinner    *spinner
	}
}

func NewGUI(ctx context.Context) *GUI {
	a := app.New()
	a.Settings().SetTheme(ui.NewDefaultTheme())
	w := a.NewWindow("Decred Eco")
	w.SetIcon(windowLogo)
	w.Resize(fyne.NewSize(1024, 768))

	mainView := ui.NewElement(&ui.Style{
		Align: ui.AlignCenter,
	})
	w.SetContent(mainView)

	ctx, cancel := context.WithCancel(ctx)

	gui := &GUI{
		ctx:      ctx,
		quit:     cancel,
		app:      a,
		driver:   a.Driver(),
		window:   w,
		mainView: mainView,
	}
	gui.logo = ui.NewSizedImage(ecoLogo, 0, 30)

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
			gui.home.dcrdProgress.SetText("Unable to retrieve Eco state: %v", err)
			select {
			case <-time.After(time.Second * 5):
			case <-ctx.Done():
				return
			}
		}
		gui.storeEcoState(&state.Eco)

		st := state.Services["dcrd"]
		dcrdLoading := st == nil

		if !dcrdLoading {
			gui.home.appRow.Show()
		}

		st = state.Services["dcrwallet"]
		walletSyncing := st == nil || st.Sync == nil || st.Sync.Progress < 0.999
		if walletSyncing {
			gui.dcrctl.spinnerBox.Show()
			gui.dcrctl.spinner.Show()
		}

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
				switch u.Service {
				case "dcrwallet":
					if walletSyncing && u.Progress > 0.999 {
						walletSyncing = false
						gui.dcrctl.spinnerBox.Hide()
						gui.dcrctl.spinner.Hide()
						canvas.Refresh(gui.dcrctl.launcher)
						gui.dex.spinnerBox.Hide()
						gui.dex.spinner.Hide()
						canvas.Refresh(gui.dex.launcher)
					}
					gui.processDCRWalletSyncUpdate(u)
				case "dcrd":
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
						// if st.On {
						// 	gui.dex.onImg.Show()
						// 	gui.dex.offImg.Hide()
						// } else {
						// 	gui.dex.onImg.Hide()
						// 	gui.dex.offImg.Show()
						// }
						canvas.Refresh(gui.dex.launcher)
					}
				case "decrediton":
					old := gui.storeDecreditonState(st)
					if old == nil || old.On != st.On {
						// if st.On {
						// 	gui.decrediton.onImg.Show()
						// 	gui.decrediton.offImg.Hide()
						// } else {
						// 	gui.decrediton.onImg.Hide()
						// 	gui.decrediton.offImg.Show()
						// }
						canvas.Refresh(gui.decrediton.launcher)
					}
				}

			},
		})
		// gui.showHomeView()
	}()

	// Run a loop to query dcrdata and update stats.
	go func() {
		// This routine should be run in a loop, or even better, the exchange
		// rate should be monitored by websocket, and the others updated in a
		// loop.

		// xcRate
		// stakeDiff
		// hashRate

		time.Sleep(250 * time.Millisecond)

		stakeDiffResp := &struct {
			Current float64 `json:"current"`
			Next    float64 `json:"next"`
		}{}
		resp, err := eco.DCRCtl(gui.ctx, "getstakedifficulty")
		if err != nil {
			log.Errorf("getstakedifficulty error: %v", err)
		} else {
			err := json.Unmarshal([]byte(resp), stakeDiffResp)
			if err != nil {
				log.Errorf("error decoding stakedifficulty response: %v", err)
			} else {
				gui.home.stakeDiff.SetText("%.2f", stakeDiffResp.Current)
				gui.home.sdDatum.Refresh()
				canvas.Refresh(gui.home.stats)
			}
		}

		var hps uint64
		resp, err = eco.DCRCtl(gui.ctx, "getnetworkhashps")
		if err != nil {
			log.Errorf("getnetworkhashps error: %v", err)
		} else {
			err := json.Unmarshal([]byte(resp), &hps)
			if err != nil {
				log.Errorf("error decoding getnetworkhashps response: %v", err)
			} else {
				gui.home.hashRate.SetText("%.2f", float64(hps)/1e15)
				gui.home.hrDatum.Refresh()
				canvas.Refresh(gui.home.stats)
			}
		}

		var blockHeight uint32
		resp, err = eco.DCRCtl(gui.ctx, "getblockcount")
		if err != nil {
			log.Errorf("getblockcount error: %v", err)
		} else {
			err := json.Unmarshal([]byte(resp), &blockHeight)
			if err != nil {
				log.Errorf("error decoding getblockcount response: %v", err)
			} else {
				gui.home.blockHeight.SetText(strconv.Itoa(int(blockHeight)))
				gui.home.bhDatum.Refresh()
				canvas.Refresh(gui.home.stats)
			}
		}

		xcResp := &struct {
			Price float64 `json:"price"`
			// lots of other info in this response if desired
		}{}
		res, err := http.Get("https://explorer.dcrdata.org/api/exchanges")
		if err != nil {
			log.Errorf("error fetching dcrdata exchange data: %v", err)
		} else {
			b, err := ioutil.ReadAll(res.Body)
			res.Body.Close()
			if err != nil {
				log.Errorf("error reading exchanges response body: %v", err)
			} else {
				err := json.Unmarshal(b, xcResp)
				if err != nil {
					log.Errorf("error decoding exchanges response: %v", err)
				} else {
					gui.home.xcRate.SetText("%.2f", float64(xcResp.Price))
					gui.home.xcDatum.Refresh()
					canvas.Refresh(gui.home.stats)
				}
			}
		}

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
	gui.quit()
	wg.Wait()
}

func (gui *GUI) setView(wgt fyne.CanvasObject) {
	gui.mainView.RemoveChildByIndex(0)
	gui.mainView.InsertChild(wgt, 0)
	gui.mainView.Refresh()
	canvas.Refresh(gui.mainView)
}

func (gui *GUI) initializeIntroView() {
	intro := widget.NewLabel(introductionText)
	intro.Wrapping = fyne.TextWrapWord

	pw := &betterEntry{Entry: &widget.Entry{}, w: 430}
	gui.intro.pw = pw
	pw.PlaceHolder = "set your password"
	pw.Password = true
	pw.ExtendBaseWidget(pw)

	gui.intro.pwRow = ui.NewElement(&ui.Style{
		Padding:      ui.FourSpec{10, 10, 10, 10},
		BgColor:      ui.InputColor,
		BorderRadius: 3,
		MaxW:         450,
	}, pw)

	bttn1 := newEcoBttn(&bttnOpts{
		bgColor:    ui.ButtonColor2,
		hoverColor: ui.ButtonHoverColor2,
	}, "Full Sync", func(*fyne.PointEvent) {
		gui.initEco(eco.SyncModeFull)
	})

	bttn2 := newEcoBttn(nil, "Lite Mode (SPV)", func(*fyne.PointEvent) {
		gui.initEco(eco.SyncModeSPV)
	})

	bttnRow := ui.NewElement(&ui.Style{
		MaxW:  450,
		Justi: ui.JustifyAround,
		Ori:   ui.OrientationHorizontal,
	},
		bttn1,
		bttn2,
	)

	gui.intro.box = ui.NewElement(
		&ui.Style{
			Spacing: 30,
			Padding: ui.FourSpec{20, 0, 0, 0},
			// maxW:    450,
			Align: ui.AlignCenter,
			// bgColor: stringToColor("#ff0"),
		},
		gui.logo,
		ui.NewLabelWithWidth(intro, 430),
		gui.intro.pwRow,
		bttnRow,
	)
}

func (gui *GUI) showIntroView() {
	gui.setView(gui.intro.box)
}

func (gui *GUI) initializeDownloadView() {

	header := ui.NewEcoLabel("Downloading", &ui.TextStyle{
		FontSize: 18,
	})

	gui.download.progress = ui.NewEcoLabel("78.6%", &ui.TextStyle{
		FontSize: 40,
		Bold:     true,
	})

	gui.download.msg = ui.NewEcoLabel("starting download...", nil)

	downloadView := ui.NewElement(
		&ui.Style{
			Spacing: 20,
			Padding: ui.FourSpec{20, 0, 20, 0},
			Align:   ui.AlignCenter,
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

	gui.home.dcrdProgress = ui.NewEcoLabel("syncing blockchain...", &ui.TextStyle{FontSize: 15})
	gui.home.dcrwProgress = ui.NewEcoLabel("syncing wallet...", &ui.TextStyle{FontSize: 15})

	newDatum := func(title string, titleSize int, el fyne.CanvasObject) *ui.Element {
		titleLbl := ui.NewElement(&ui.Style{
			Display:  ui.DisplayInline,
			Position: ui.PositionAbsolute,
			Left:     intptr(5),
			Top:      intptr(-13),
			BgColor:  ui.BgColor,
			Padding:  ui.FourSpec{0, 3, 0, 3},
		},
			ui.NewEcoLabel(title, &ui.TextStyle{FontSize: titleSize, Bold: true}),
		)
		datum := ui.NewElement(&ui.Style{
			Display:      ui.DisplayInline,
			BorderWidth:  1,
			BorderRadius: 3,
			BgColor:      ui.BgColor,
			Padding:      ui.FourSpec{10, 20, 10, 20},
			BorderBox:    true,
		},
			titleLbl,
			el,
		)

		// datum.Name = "datum"
		return datum
	}

	dcrdDatum := newDatum("dcrd", 15, gui.home.dcrdProgress)
	dcrdDatum.Style.Width = "48%"

	dcrwDatum := newDatum("dcrwallet", 15, gui.home.dcrwProgress)
	dcrwDatum.Style.Width = "48%"

	progressRow := ui.NewElement(&ui.Style{
		Ori:     ui.OrientationHorizontal,
		Justi:   ui.JustifyBetween,
		Margins: ui.FourSpec{10, 0, 5, 0},
	},
		dcrdDatum,
		dcrwDatum,
	)

	var logoHeight uint = 35

	makeAppLauncher := func(desc string, click func(*fyne.PointEvent), els ...fyne.CanvasObject) *ui.Element {
		els = append(els, ui.NewEcoLabel(desc, nil))
		return ui.NewElement(&ui.Style{
			Align:        ui.AlignCenter,
			Padding:      ui.FourSpec{10, 15, 10, 15},
			Cursor:       desktop.PointerCursor,
			Display:      ui.DisplayInline,
			BgColor:      ui.StringToColor("#0a0a0a"),
			BorderRadius: 2,
			BorderWidth:  1,
			BorderColor:  ui.StringToColor("#333"),
			Listeners: ui.EventListeners{
				Click: click,
			},
			Spacing: 5,
		},
			els...,
		)
	}

	newSpinnerBox := func() (*ui.Element, *spinner) {
		var zero int
		spinner := newSpinner(gui.ctx, 7, 40, ui.DecredKeyBlue, ui.DecredGreen)
		spinnerBox := ui.NewElement(&ui.Style{
			Position: ui.PositionAbsolute,
			Left:     &zero,
			Right:    &zero,
			Top:      &zero,
			Bottom:   &zero,
			Ori:      ui.OrientationHorizontal,
			Align:    ui.AlignMiddle,
			Justi:    ui.JustifyAround,
			BgColor:  ui.BlockerColor,
		},
			spinner,
		)
		spinnerBox.Hide()
		// spinner is hidden by default
		return spinnerBox, spinner
	}

	dec := &gui.decrediton
	dec.launcher = makeAppLauncher("Full-featured wallet", func(*fyne.PointEvent) {
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
		ui.NewSizedImage(decreditonLogo, 0, logoHeight),
	)

	// DEX
	dex := &gui.dex
	gui.dex.spinnerBox, gui.dex.spinner = newSpinnerBox()
	dex.launcher = makeAppLauncher("Trade crypto", func(*fyne.PointEvent) {
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
		ui.NewSizedImage(dexLogo, 0, logoHeight),
		gui.dex.spinnerBox,
	)
	gui.dex.launcher.Hide()

	// DCRCtl+
	ctl := &gui.dcrctl
	gui.dcrctl.spinnerBox, gui.dcrctl.spinner = newSpinnerBox()
	ctl.launcher = makeAppLauncher("Query Decred", func(*fyne.PointEvent) {
		gui.showDCRCtl()
	},
		ui.NewSizedImage(dcrctlLogo, 0, logoHeight),
		gui.dcrctl.spinnerBox,
	)

	// A horizontal div (with wrapping?) holding image buttons to start various
	// Decred services.
	gui.home.appRow = ui.NewElement(&ui.Style{
		Ori:     ui.OrientationHorizontal,
		Justi:   ui.JustifyStart,
		Align:   ui.AlignCenter,
		Spacing: 20,
	},
		dec.launcher,
		dex.launcher,
		ctl.launcher,
	)
	gui.home.appRow.Hide()

	// gui.home.appRow.Name = "appRow"

	sectionHeader := func(s string) *ui.Element {
		return ui.NewElement(&ui.Style{
			Ori: ui.OrientationHorizontal,
		},
			ui.NewEcoLabel(s, &ui.TextStyle{FontSize: 18, Bold: true}),
		)
	}

	valueWithUnit := func(val fyne.CanvasObject, unit string) *ui.Element {
		return ui.NewElement(&ui.Style{
			Ori:     ui.OrientationHorizontal,
			Display: ui.DisplayInline,
			Spacing: 5,
			// Would be nice to do AlignBaseline, but it seems fyne.MeasureText
			// -> (*canvas.Text).Size() returns a measurement that is based on
			// the font family rather than the actual text being displayed, so
			// the text can have some vertical padding, making alignment.
			// Next best choice is AlignMiddle, which is the default.
			// difficult.
			// Align:   ui.AlignBaseline,
		},
			val,
			ui.NewEcoLabel(unit, &ui.TextStyle{FontSize: 12, Bold: true, Color: ui.StringToColor("#777")}),
		)
	}

	datumStyle := &ui.TextStyle{FontSize: 17}

	gui.home.xcRate = ui.NewEcoLabel("...", datumStyle)
	gui.home.xcDatum = newDatum("Exchange Rate", 13, valueWithUnit(gui.home.xcRate, "USD"))

	gui.home.stakeDiff = ui.NewEcoLabel("...", datumStyle)
	gui.home.sdDatum = newDatum("Ticket Price", 13, valueWithUnit(gui.home.stakeDiff, "DCR"))

	gui.home.hashRate = ui.NewEcoLabel("...", datumStyle)
	gui.home.hrDatum = newDatum("Hashrate", 13, valueWithUnit(gui.home.hashRate, "Ph/s"))

	gui.home.blockHeight = ui.NewEcoLabel("...", datumStyle)
	gui.home.bhDatum = newDatum("Blocks", 13, gui.home.blockHeight)

	gui.home.stats = ui.NewElement(&ui.Style{
		Ori:     ui.OrientationHorizontal,
		Justi:   ui.JustifyStart,
		Spacing: 20,
		Margins: ui.FourSpec{10, 0, 10, 0},
	},
		gui.home.xcDatum, gui.home.sdDatum, gui.home.hrDatum, gui.home.bhDatum,
	)

	// topHR := ui.NewHorizontalRule(1, ui.DefaultBorderColor, 5)
	// topHR.Style.Margins[0] = 20

	gui.home.box = ui.NewElement(
		&ui.Style{
			Padding: ui.FourSpec{20, 0, 0, 0},
			Align:   ui.AlignCenter,
			Spacing: 5,
			MaxW:    900,
			// expandVertically: true,
			// justi:            justifyStart,
		},
		gui.logo,
		// topHR,
		sectionHeader("Stats"),
		gui.home.stats,
		ui.NewHorizontalRule(1, ui.DefaultBorderColor, 5),
		sectionHeader("Apps"),
		gui.home.appRow,
		ui.NewHorizontalRule(1, ui.DefaultBorderColor, 5),
		sectionHeader("Sync"),
		progressRow,
	)
	// gui.home.box.Name = "homeBox"
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

	var link *ui.Element
	link = ui.NewElement(&ui.Style{
		Padding: ui.FourSpec{-1, 5, -1, 5},
		BgColor: ui.InputColor,
		Cursor:  desktop.PointerCursor,
		Ori:     ui.OrientationHorizontal,
		Display: ui.DisplayInline,
		Spacing: 5,
		Listeners: ui.EventListeners{
			Click: func(ev *fyne.PointEvent) {
				gui.showHomeView()
			},
			MouseIn: func(*desktop.MouseEvent) {
				link.SetBackgroundColor(ui.DefaultButtonColor)
			},
			MouseOut: func() {
				link.SetBackgroundColor(ui.InputColor)
			},
		},
	}, larrow, goHome)

	linkRow := ui.NewElement(&ui.Style{
		Align:   ui.AlignLeft,
		Display: ui.DisplayInline,
		MinW:    750,
	},
		link,
	)

	var resultDiv *ui.Element
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
		resp, err := eco.DCRCtl(gui.ctx, strings.TrimSpace(input.Text))
		if err == nil {
			gui.dcrctl.results.SetText(fmt.Sprintf("result for %q:\n%s", input.Text, resp))
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

	resultDiv = ui.NewElement(&ui.Style{
		BgColor:      ui.InputColor,
		Padding:      ui.FourSpec{10, 10, 10, 10},
		Margins:      ui.FourSpec{1, 1, 20, 1},
		BorderRadius: 4,
		BorderWidth:  1,
		BorderColor:  ui.StringToColor("#444"),
		Display:      ui.DisplayInline,
		MinW:         730,
	},
		gui.dcrctl.results,
	)
	resultDiv.Hide()

	inputElement := ui.NewElement(&ui.Style{
		Padding:      ui.FourSpec{10, 10, 10, 10},
		BgColor:      ui.InputColor,
		BorderRadius: 3,
		MaxW:         750,
	}, input)

	inputElement.Name = "inputElement"

	gui.dcrctl.view = ui.NewElement(
		&ui.Style{
			Padding: ui.FourSpec{20, 0, 0, 0},
			Align:   ui.AlignCenter,
			Spacing: 15,
		},
		ui.NewSizedImage(dcrctlLogo, 0, 30),
		linkRow,
		inputElement,
		resultDiv,
	)

	gui.dcrctl.view.Name = "inputView"
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
	gui.updateSyncLabel(gui.home.dcrdProgress, u)
}

func (gui *GUI) processDCRWalletSyncUpdate(u *eco.Progress) {
	gui.updateSyncLabel(gui.home.dcrwProgress, u)
}

func (gui *GUI) updateSyncLabel(lbl *ui.EcoLabel, u *eco.Progress) {
	if u.Err != "" {
		lbl.SetText("sync error: %s", u.Err)
		return
	}
	lbl.SetText("%s (%.0f%%)", u.Status, u.Progress*100)
	lbl.Refresh()
	gui.home.box.Refresh()
	canvas.Refresh(lbl)
}

// initEco should be run in a goroutine.
func (gui *GUI) initEco(syncMode eco.SyncMode) {
	pw := gui.intro.pw.Text
	ch, err := eco.Init(gui.ctx, pw, syncMode)
	if err != nil {
		gui.download.msg.SetText("Error initalizing Eco: %v", err)
		return
	}
	gui.showDownloadView()
	for {
		select {
		case u := <-ch:
			if err != nil {
				gui.download.msg.SetText(err.Error())
				return
			}
			if u.Err != "" {
				gui.download.msg.SetText(u.Err)
				return
			}
			gui.download.msg.SetText(u.Status)
			gui.download.progress.SetText("%.0f%%", u.Progress*100)
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
	return ui.Transparent
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

func intptr(i int) *int {
	return &i
}
