package buildlog

import (
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
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
	}
	return logger
}

func initLogger() {
	mx.Lock()
	defer mx.Unlock()

	w := out()
	logger = slog.New(tint.NewHandler(w, &tint.Options{
		Level:      levelFor(verbosity),
		TimeFormat: "",
		NoColor:    !isatty.IsTerminal(w.Fd()),
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if len(groups) == 0 && attr.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return attr
		},
	}))
}

func levelFor(v int) slog.Level {
	switch {
	case v > 0:
		return slog.LevelDebug
	case v == 0:
		return slog.LevelInfo
	case v == -1:
		return slog.LevelWarn
	default:
		return slog.LevelError
	}
}
