package plog

import (
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"
)

func Event(str string) {
	Logger().Debug(str)
}
func Config(key, value string) {
	Logger().Warn("config", slog.String(key, value))
}
func Fatalln(err error) {
	Logger().Error(err.Error())
	os.Exit(1)
}

func File(action, filename string) {
	Logger().Info(action, slog.String("filename", filename))
}
func Dir(action, dir, builder string) {
	Logger().Info(action,
		slog.String("dir", dir),
		slog.String("builder", builder),
	)
}
func Command(cmd string, args ...string) {
	Logger().Info("command", slog.String("cmd", cmd+" "+strings.Join(args, " ")))
}
func Image(img string) {
	Logger().Info("publish", slog.String("image", img))
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
