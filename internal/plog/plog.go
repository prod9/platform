package plog

import (
	"io"
	"log/slog"
	"os"

	"github.com/pterm/pterm"
)

var (
	logger *slog.Logger
	opts   *slog.HandlerOptions
)

func out() *os.File { return os.Stderr }

func SetQuietness(q int) {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	switch {
	case q >= 2:
		opts.Level = slog.LevelError
	case q == 1:
		opts.Level = slog.LevelWarn
	case q == 0:
		opts.Level = slog.LevelInfo
	case q < 0:
		opts.Level = slog.LevelDebug
	}
}

func OutputForDagger() io.Writer {
	if opts != nil && opts.Level.Level() <= slog.LevelInfo {
		return out()
	} else {
		return io.Discard
	}
}

func Logger() *slog.Logger {
	if logger == nil {
		if opts == nil {
			opts = &slog.HandlerOptions{}
		}

		opts.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == "time" {
				return slog.Attr{Key: "time", Value: slog.Value{}}
			} else {
				return a
			}
		}

		ptLogger := pterm.DefaultLogger
		ptLogger.ShowTime = false
		ptLogger.Writer = out()

		logger = slog.New(pterm.NewSlogHandler(&ptLogger))
	}

	return logger
}
