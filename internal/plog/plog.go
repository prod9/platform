package plog

import (
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/pterm/pterm"
)

var (
	mx sync.Mutex

	logger    *slog.Logger
	verbosity int
)

func out() *os.File { return os.Stderr }

func SetVerbosity(v int) {
	mx.Lock()
	defer mx.Unlock()

	verbosity = v
	logger = nil
}

func OutputForDagger() io.Writer {
	if verbosity >= 0 {
		return out()
	} else {
		return io.Discard
	}
}

func Logger() *slog.Logger {
	if logger == nil {
		initLogger()
		return Logger()
	} else {
		return logger
	}
}
func initLogger() {
	mx.Lock()
	defer mx.Unlock()

	ptLogger := pterm.DefaultLogger
	ptLogger.ShowTime = false
	ptLogger.Writer = out()

	switch {
	case verbosity > 0:
		ptLogger.Level = pterm.LogLevelDebug
	case verbosity == 0:
		ptLogger.Level = pterm.LogLevelInfo
	case verbosity == -1:
		ptLogger.Level = pterm.LogLevelWarn
	default:
		ptLogger.Level = pterm.LogLevelError
	}

	logger = slog.New(pterm.NewSlogHandler(&ptLogger))
}
