<!-- not spec/decision because: a capability-gap handoff to the fx agent; the ruling and the
fix belong to the fx repo, not this one -->

# Handoff to fx: fxlog can't resolve slog LogValuer / groups

For the **fx agent**. Discovered while fixing platform's undebuggable build errors; the
root capability gap is in `fxlog` and affects every fx project that logs enriched errors.

## Problem

slog's idiomatic way for a value to expose structured fields is `slog.LogValuer`
(`LogValue() slog.Value`, typically returning a `GroupValue`). Resolution is the
**handler's** responsibility — it must call `attr.Value.Resolve()`. fxlog's sinks don't:

- **`ZerologSink.Log`** (the default sink; `LOG_SINK` defaults to `"zerolog"`) switches on
  `attr.Value.Kind()` and explicitly `bail`s on **`slog.KindGroup`** and
  **`slog.KindLogValuer`** ("not supported"). So a LogValuer never fires and groups are
  dropped.
- **`ZerologSink.Error(err)`** is `.Err(err).Msg("error")` — renders only `err.Error()`,
  no field extraction.
- **`SLogSink`** is a thin passthrough (`LogAttrs` to a wrapped `*slog.Logger`); it resolves
  nothing itself — behavior is entirely the underlying handler's.

(Source: `fx.prodigy9.co@v0.8.6/fxlog/zerolog_sink.go`, `slog_sink.go`.)

Consequence: an fx app can't hand slog a structured error and get its fields rendered. The
error's detail (e.g. a wrapped subprocess's cmd/exit/stderr) is flattened to the terse
`err.Error()` string. This is exactly what left platform's build failures showing only
`exit code: 1` with the real stderr invisible.

## What platform is doing locally

1. A LogValuer error type at an un-forgettable chokepoint (the build engine) that wraps the
   underlying error and returns `slog.GroupValue(cmd, exit, stderr, stdout)` from
   `LogValue()`.
2. A hand-written `slog.Handler` that **`Resolve()`s** every attr value, renders groups,
   colors by level (ANSI, auto-off when the sink isn't a TTY so pipes/CI stay clean), and
   gates on verbosity. Replaces pterm, which had the same gaps as fxlog's zerolog sink
   (no resolve, `WithGroup` no-op) plus a TTY renderer that fought other stdout writers.

## The ask

Consider fxlog owning a **first-class resolving + color console slog sink** — one that
`Resolve()`s LogValuers, renders (or sensibly flattens) groups, and does TTY-aware color.
If fxlog provides it, platform drops its bespoke handler and converges on the fx sink, and
every other fx project gets debuggable structured errors for free. The LogValuer-error
pattern (1) is a project-level convention; the resolving-sink capability (2) is the piece
that belongs upstream.
