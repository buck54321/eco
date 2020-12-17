module github.com/buck54321/eco/ecogui

go 1.15

replace (
	fyne.io/fyne => ../../../fyne
	github.com/buck54321/eco => ../../
	github.com/buck54321/eco/ui => ../../ui
)

require (
	fyne.io/fyne v1.4.2
	github.com/buck54321/eco v0.0.0-20201207140308-580c96d49dac
	github.com/buck54321/eco/ui v0.0.0-00010101000000-000000000000
	github.com/decred/slog v1.1.0
	github.com/disintegration/imaging v1.6.2
	github.com/goki/mat32 v1.0.2
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/robfig/graphics-go v0.0.0-20140921172951-05ad18ff57b8
)
