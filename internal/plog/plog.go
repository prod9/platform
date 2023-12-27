package plog

import (
	"io"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
)

var (
	logger *slog.Logger
	opts   *slog.HandlerOptions
)

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
		return os.Stderr
	} else {
		return io.Discard
	}
}

func Logger() *slog.Logger {
	if logger == nil {
		if opts == nil {
			opts = &slog.HandlerOptions{}
		}

		// TODO: Auto detect interactive TTY?
		logger = slog.New(tint.NewHandler(os.Stderr, &tint.Options{
			NoColor:    !isatty.IsTerminal(os.Stderr.Fd()),
			TimeFormat: " ", // adds space to disable time output
		}))
	}

	return logger
}
