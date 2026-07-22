package buildlog

import (
	"log/slog"
	"os"
	"strings"
	"time"
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

// Git traces a git invocation (`git <args>`) at Debug level — visible only
// at raised verbosity, for diagnosing what platform ran under the hood.
func Git(args ...string) {
	Logger().Debug("git", slog.String("args", strings.Join(args, " ")))
}

// File reports a file action taken on the operator's tree (`write x`,
// `overwrite y` — the scaffold apply trail) at Info level.
func File(action, filename string) {
	Logger().Info(action, slog.String("filename", filename))
}

// StepStart announces a build step beginning (`step deps`) at Info level. It is what
// makes a long step visible while it runs — the operator sees the name before the wait,
// not after it.
func StepStart(unit, step string) {
	Logger().Info("step",
		slog.String("unit", unit),
		slog.String("step", step),
	)
}

// StepDone closes the step StepStart announced, carrying what it cost and, when it
// failed, what it failed with — at Error level then, so a failure is not one Info line
// among many.
func StepDone(unit, step string, took time.Duration, err error) {
	attrs := []any{
		slog.String("unit", unit),
		slog.String("step", step),
		slog.Duration("took", took),
	}
	if err != nil {
		Logger().Error("step failed", append(attrs, slog.Any("err", enrichErr(err)))...)
		return
	}
	Logger().Info("step done", attrs...)
}

// BuildDone reports one unit's build finishing, at Info or Error by outcome. Under
// fan-out each unit reports its own, so the unit name is the only way to tell them apart.
func BuildDone(unit string, took time.Duration, err error) {
	attrs := []any{
		slog.String("unit", unit),
		slog.Duration("took", took),
	}
	if err != nil {
		Logger().Error("build failed", append(attrs, slog.Any("err", enrichErr(err)))...)
		return
	}
	Logger().Info("build done", attrs...)
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
