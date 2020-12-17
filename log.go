package eco

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/buck54321/eco/db"
	"github.com/decred/slog"
	"github.com/jrick/logrotate/rotator"
)

var (
	// logRotator is one of the logging outputs. It should be closed on
	// application shutdown.
	backendLog  = slog.NewBackend(logWriter{}, slog.WithFlags(slog.LUTC))
	logRotator  *rotator.Rotator
	log         = slog.Disabled
	maxLogRolls = 16
)

// logWriter implements an io.Writer that outputs to stdout
// and a rotating log file.
type logWriter struct{}

// Write writes the data in p to both destinations.
func (w logWriter) Write(p []byte) (n int, err error) {
	os.Stdout.Write(p)
	return logRotator.Write(p)
}

// InitLogging initializes the logging rotater to write logs to logFile and
// create roll files in the same directory. initLogging must be called before
// the package-global log rotator variables are used.
func InitLogging(name string) slog.Logger {
	logDir := filepath.Join(AppDir, "eco", "logs")
	err := os.MkdirAll(logDir, 0700)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create log directory: %v\n", err)
		os.Exit(1)
	}

	logFilename := name + ".log"
	fmt.Println("--Creating log rotator at", filepath.Join(logDir, logFilename))

	logRotator, err = rotator.New(filepath.Join(logDir, logFilename), 32*1024, false, maxLogRolls)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create file rotator: %v\n", err)
		os.Exit(1)
	}
	log = backendLog.Logger("ECO")
	db.UseLogger(backendLog.Logger("DB"))
	return backendLog.Logger(strings.ToUpper(name))
}

// UseLogger can be used to set the logger for users of package functions that
// aren't initalizing an Eco.
func UseLogger(logger slog.Logger) {
	log = logger
}

// closeFileLogger closes the log rotator.
func closeFileLogger() {
	if logRotator != nil {
		logRotator.Close()
	}
}
