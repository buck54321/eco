module github.com/buck54321/eco

go 1.15

require (
	decred.org/dcrdex v0.1.2
	decred.org/dcrwallet v1.6.0-rc3
	fyne.io/fyne v1.4.2
	github.com/decred/dcrd/certgen v1.1.1
	github.com/decred/dcrd/chaincfg/v3 v3.0.0
	github.com/decred/dcrd/dcrutil v1.4.0
	github.com/decred/dcrd/dcrutil/v2 v2.0.1
	github.com/decred/dcrd/rpc/jsonrpc/types/v2 v2.3.0
	github.com/decred/dcrd/rpcclient/v5 v5.0.0
	github.com/decred/dcrd/rpcclient/v6 v6.0.2
	github.com/decred/dcrwallet/rpc/jsonrpc/types v1.4.0
	github.com/decred/slog v1.1.0
	github.com/go-chi/chi v4.0.2+incompatible
	github.com/goki/gi v1.1.2
	github.com/goki/ki v1.0.5
	github.com/goki/mat32 v1.0.2
	github.com/jrick/logrotate v1.0.0
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	go.etcd.io/bbolt v1.3.5
	golang.org/x/crypto v0.0.0-20200820211705-5c72a883971a
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b
)

replace github.com/buck54321/eco/db => ../db
