package buildlog

import (
	"log/slog"
	"os"
	"strings"
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

func Git(cmd string, args ...string) {
	Logger().Debug("git", cmd, strings.Join(args, " "))
}
func GitInfo(item, value string) {
	Logger().Info("git", item, value)
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
