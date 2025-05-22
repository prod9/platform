package plog

import (
	"log/slog"
	"net/url"
	"os"
	"time"
)

func Event(str string) {
	Logger().Debug(str)
}
func Config(key, value string) {
	Logger().Warn("config", slog.String(key, value))
}
func Error(err error) {
	Logger().Error(err.Error())
}
func Fatalln(err error) {
	Error(err)
	os.Exit(1)
}

func Git(action, hash string) {
	Logger().Info("git", action, hash)
}
func File(action, filename string) {
	Logger().Info(action, slog.String("filename", filename))
}
func Image(action, image, hash string) {
	Logger().Info(action,
		slog.String("hash", hash),
		slog.String("image", image),
	)
}

func HTTPServing(addr string) {
	Logger().Info("serving",
		slog.String("addr", addr))
}
func HTTPRequest(
	method string,
	url *url.URL,
	code int,
	duration time.Duration,
	written int64,
) {
	Logger().Info("request",
		slog.String("method", method),
		slog.String("url", url.Path),
		slog.Int("code", code),
		slog.Duration("d", duration),
		slog.Int64("written", written),
	)
}
