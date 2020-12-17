module github.com/buck54321/eco/cmd/installer

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
	google.golang.org/appengine v1.4.0
)
