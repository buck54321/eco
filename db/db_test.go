package db

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/decred/slog"
)

var (
	tDir     string
	tCtx     context.Context
	tCounter int
	tLogger  = slog.NewBackend(os.Stdout).Logger("TEST")
)

func newTestDB(t *testing.T) (*DB, func()) {
	tCounter++
	dbPath := filepath.Join(tDir, fmt.Sprintf("db%d.db", tCounter))
	db, err := NewDB(dbPath, tLogger)
	if err != nil {
		t.Fatalf("error creating dB: %v", err)
	}
	return db, func() {
		db.Close()
	}
}

func TestMain(m *testing.M) {
	defer os.Stdout.Sync()
	doIt := func() int {
		var err error
		tDir, err = ioutil.TempDir("", "dbtest")
		if err != nil {
			fmt.Println("error creating temporary directory:", err)
			return -1
		}
		defer os.RemoveAll(tDir)
		var shutdown func()
		tCtx, shutdown = context.WithCancel(context.Background())
		defer shutdown()
		return m.Run()
	}
	os.Exit(doIt())
}

func TestStoreFetch(t *testing.T) {
	db, done := newTestDB(t)
	defer done()
	type settings struct {
		A int
		B int
	}
	settingsIn := &settings{
		A: 1,
		B: 2,
	}
	svc := "dcrdctldexwalletlnd"
	err := db.EncodeStore(svc, settingsIn)
	if err != nil {
		t.Fatalf("SaveServiceSettings error: %v", err)
	}
	settingsOut := new(settings)
	loaded, err := db.FetchDecode(svc, settingsOut)
	if err != nil {
		t.Fatalf("LoadServiceSettings error: %v", err)
	}
	if !loaded {
		t.Fatalf("Failed to load settings")
	}
	if settingsIn.A != settingsOut.A {
		t.Fatalf("Wrong settings.A out. Wanted %d, got %d.", settingsIn.A, settingsOut.A)
	}
	if settingsIn.B != settingsOut.B {
		t.Fatalf("Wrong settings.B out. Wanted %d, got %d.", settingsIn.B, settingsOut.B)
	}
}
