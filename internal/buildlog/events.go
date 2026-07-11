package buildlog

import (
	"log/slog"
	"os"
	"strings"
)

// Every log platform emits goes through one of the typed constructors below — log
// output is consolidated and grouped by construction, not filtered after the fact.
// A new kind of output means a new method here, never an ad-hoc Logger() call at
// the emit site.

// Event narrates free-form progress ("pruning dagger build cache", "exited") at
// Debug level — invisible at default verbosity. Never use it for warnings or
// operator-actionable notes; those need a typed, visible constructor.
func Event(str string) {
	Logger().Debug(str)
}

// Config reports an effective-config fact the operator should notice — an env
// override taking effect, a deprecated key being read — as `config <key>=<value>`
// at Warn level, visible at default verbosity.
func Config(key, value string) {
	Logger().Warn("config", slog.String(key, value))
}

// Error reports a failure at Error level, attaching the enriched error (exec
// errors gain captured output) as a structured attr.
func Error(err error) {
	Logger().Error(err.Error(), slog.Any("err", enrichErr(err)))
}

// Fatalln is Error followed by exit(1) — the CLI's terminal failure path.
func Fatalln(err error) {
	Error(err)
	os.Exit(1)
}

// Git traces a git invocation (`git <cmd> <args>`) at Debug level — visible only
// at raised verbosity, for diagnosing what platform ran under the hood.
func Git(cmd string, args ...string) {
	Logger().Debug("git", cmd, strings.Join(args, " "))
}

// GitInfo reports a resolved git fact (current branch, tracking remote, the tag
// just cut) at Info level.
func GitInfo(item, value string) {
	Logger().Info("git", item, value)
}

// File reports a file action taken on the operator's tree (`write x`,
// `overwrite y` — the scaffold apply trail) at Info level.
func File(action, filename string) {
	Logger().Info(action, slog.String("filename", filename))
}

// Image reports an image action (built, published) with its ref and digest at
// Info level — the delivery audit trail.
func Image(action, image, hash string) {
	Logger().Info(action,
		slog.String("hash", hash),
		slog.String("image", image),
	)
}

// HTTPServing reports the address a long-running server bound at Info level.
func HTTPServing(addr string) {
	Logger().Info("serving",
		slog.String("addr", addr))
}
