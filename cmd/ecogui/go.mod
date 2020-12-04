module github.com/buck54321/eco/ecogui

go 1.15

replace (
	github.com/buck54321/eco => ../../
	fyne.io/fyne => ../../../fyne
)

require (
	fyne.io/fyne v1.4.1
	github.com/buck54321/eco v0.0.0-00010101000000-000000000000
	github.com/decred/slog v1.1.0
	github.com/goki/mat32 v1.0.2
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
)
