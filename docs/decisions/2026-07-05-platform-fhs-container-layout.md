# Platform FHS container layout; cmd is the runtime command, not the binary name

- **Date:** 2026-07-05
- **PR:** manual
- **Status:** accepted

## Decision

Every built container lays down a small platform-owned FHS-style tree:

| Path            | Role                                                          |
|-----------------|--------------------------------------------------------------|
| `/platform/src` | build workspace — host sources are copied here and compiled  |
| `/platform/bin` | compiled executables, on `PATH`                              |
| `/platform/run` | runtime working directory (cwd) — assets and data live here  |

The Go build output is named after the **module** (`unit.Name`) and installed to
`/platform/bin/<Name>`; the container runs it bare by name (resolved on `PATH`). `cmd`
(`CommandName`) is the **runtime command** — an optional override of the default entrypoint —
never the build-output name. Its default is the module-named binary (Go) or the interpreter
(`node`/`caddy` for pnpm). `/app` and `/out` are retired.

## Rationale

The obvious layout — build the binary to `/app/<cmd>` and name it from `CommandName` — was
what platform did, and it was wrong on two counts a future reader would otherwise re-derive:

- **`cmd` as binary name is a conflation.** `CommandName` was literally `BinaryName` until a
  2023 rename (`efe5122`) that added an `Entrypoint` field to *split* command-from-binary —
  then deleted `Entrypoint` hours later (`fad8ed4`) without finishing the split. So the field
  says "command" (and the struct groups it under container/runtime settings with `env`/`port`/
  `args`), while the Go builders kept using it to name the compiled artifact. pnpm already
  treats `cmd` as a runtime command (`node`, the interpreter — JS compiles nothing), so making
  `cmd` uniformly "a command on `PATH`" is the only cross-builder-consistent meaning. The Go
  binary name comes from build inputs (the module) instead.

- **Binary and source can't share a directory.** A module `api` built to `/app/api` collides
  with an `api/` package directory in the same tree. Splitting into three trees —
  source in `/platform/src`, executables in `/platform/bin`, data in `/platform/run` —
  removes the collision structurally rather than by naming discipline.

The FHS shape is also chosen for **operator debuggability**: someone shelling into a running
container lands in `/platform/run` (their data), has `/platform/bin` on `PATH` (their
commands), and finds sources under `/platform/src` — one predictable namespace, versus hunting
through `/usr/local/bin` mixed with system binaries.

## Consequence

Behavior change, absorbed as intended smoke drift: Go testbeds drop their now-redundant `cmd`;
`bootstrap` no longer emits `cmd` for native modules; the Go binary runs bare by name (no `./`
prefix). All builds are otherwise transparent — the re-recorded golden moved only on the
bootstrap `cmd` emission. The `dockerfile` builder is unaffected (it owns its own base and
`FROM`).
