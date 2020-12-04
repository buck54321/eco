module github.com/buck54321/eco/ecogui-old

go 1.15

replace (
	github.com/buck54321/eco => ../../
	github.com/goki/gi => ../../../gi
)

require (
	github.com/buck54321/eco v0.0.0-00010101000000-000000000000
	github.com/decred/slog v1.1.0
	github.com/goki/gi v1.1.2
	github.com/goki/ki v1.0.5
	github.com/goki/mat32 v1.0.2
)
