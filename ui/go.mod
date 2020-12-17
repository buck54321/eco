module github.com/buck54321/eco/ui

go 1.15

replace (
	fyne.io/fyne => ../../fyne
	github.com/buck54321/eco => ../
)

require (
	fyne.io/fyne v1.4.2
	github.com/buck54321/eco v0.0.0-20201207140308-580c96d49dac
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
)
