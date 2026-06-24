# Split logging into build (buildlog) and platform-server (fxlog) channels

- **Date:** 2026-06-24
- **PR:** manual
- **Status:** accepted (resolves roadmap #5, "plog → fxlog")

## Decision

Platform has **two distinct log channels**, owned by two packages:

- **`internal/buildlog`** (renamed from `internal/plog`) — the **CLI / build-pipeline
  console** channel. Owns `-q`/`-v` verbosity (`SetVerbosity` + pterm levels), the Dagger
  `WithLogOutput` writer (`OutputForDagger`), the build events (`Image`/`File`/`Git`/
  `Config`/`Event`), and the CLI error/exit path (`Fatalln`/`Error`). Stays pretty-printed to
  stderr — ephemeral, human-facing, verbosity-gated.
- **`fx.prodigy9.co/fxlog`** — the **platform-server** channel. Structured, sink-swappable
  (zerolog default, `LOG_SINK=slog`, or any `SetSink`). Carries the logs of platform's own
  long-running HTTP server components: `vanity` today, the Phase B control-plane API + web UI
  later.

The boundary is **who emits**: an HTTP server component that *is* platform → `fxlog`;
everything the CLI build tool does to a target repo, plus its own process lifecycle →
`buildlog`. (`preview` serving a built container over a Dagger tunnel is build-side; `vanity`
serving real redirect traffic is server-side, even though both once shared `HTTPServing`.)

## Rationale

Roadmap #5 was written as a wholesale swap — replace `internal/plog` with `fxlog`, map
`SetVerbosity` to "fxlog's level control," bridge `OutputForDagger` to "a writer or slog
handler fxlog exposes." Reading `fxlog` v0.8.6 falsified both premises:

- **fxlog has no log levels, by design.** Its `sink.go` states the convention is "*not doing
  the complex log-level juggling*." There is no Debug/Info/Warn/Error split and no
  `SetVerbosity` equivalent — so `-q`/`-v` has nowhere to land.
- **fxlog exposes no `io.Writer`.** Its only extension point is `SetSink(Sink)` where
  `Sink = { Log; Error }`. Dagger's `WithLogOutput` needs an `io.Writer`; fxlog cannot
  produce one.

The insight is **not** "fxlog is missing features." It is that verbosity and a console writer
are **build/CLI concerns, not server concerns**. Level-gated pretty output for a human at a
terminal belongs to the build tool; structured, shippable, level-less records belong to the
server. So #5 is a **separation**, not a swap — and the separation is what makes fxlog fit:
it came from the fx web framework, has request logging built in, and its sink is the "move
the logs somewhere proper" seam the Phase B API + web UI will share.

Scope landed now: `vanity` (platform's one HTTP server today) moved entirely onto fxlog;
`HTTPRequest` left `buildlog` with it (no build-channel caller remained), `HTTPServing` stayed
for `preview`. `plog` was renamed to `buildlog` so the channel names itself and contrasts with
fxlog. No Phase B server code exists yet, so nothing else moved.

## Consequence

`vanity` loses `-q`/`-v` gating on its request logs (fxlog is level-less). That is consistent
with the intent — server logs go to a sink, not a verbosity-gated console — and is the only
behavior change. The build channel keeps `-q`/`-v`, the Dagger writer, and pterm output intact.
