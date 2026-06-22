# Render via the linked CUE engine, not the `cue` binary

- **Date:** 2026-06-23
- **PR:** manual
- **Status:** accepted

## Decision

`ops render` evaluates the apps CUE package through the **linked CUE library**
(`cuelang.org/go`, pinned `@v0.15.4` in `go.mod`) — `cue/load` + `cuecontext` +
`encoding/yaml` + a `mod/modconfig` registry — instead of shelling out to a `cue` binary on
`$PATH`. The pinned library version is the single source of CUE-engine truth for platform.

## Rationale

`render.go` previously ran `exec.Command("cue", "export", …)`. That made the **CUE engine
version ambient** — whatever `cue` happened to be on the machine's `$PATH`. On a dev box that
was v0.16.1, and per the defs mixin ADR (`prod9/infra-defs`) **v0.16.1 panics on the defs
`parts` package** — defs is built for v0.15.4. So `cue export` of any defs-based app (the
dagger engine, or `../infra`'s apps) would panic for a reason invisible in our code.

The obvious smaller fix — *vendor/pin a `cue` binary* (à la `infra-defs/cue.sh`) — was
**rejected**: it still shells an external process, still needs the binary fetched/installed
(a global-state mutation), and keeps a second toolchain to manage. Linking the library
instead:

- **Pins the engine version in `go.mod`** — one dependency graph, no `$PATH`/binary drift, and
  we move it deliberately (in lockstep with defs) rather than inheriting the machine's `cue`.
- **No subprocess, no external binary** — render is pure in-process Go.
- **Folds in B3b** — the same library does `cue.mod` scaffolding via `mod/modfile`, so init
  never shells `cue mod init/get` nor hand-writes `module.cue` (no format lock-in).

chakrit's call on the dependency weight: "heavy cue dependency import is kind of fine as
that's our core for a lot of things, so it's warranted."

## Consequences

- `exportCue` (was `exportApps`) uses `load.Instances` + `cuecontext.BuildInstance` +
  `cueyaml.Encode`; image injection via `load.Config.Tags`; registry via
  `modconfig.NewRegistry{CUERegistry: $CUE_REGISTRY or DefaultRegistry}`. `buildTree` and the
  emitted YAML shape are unchanged. `exec.Command`/`registryEnv` removed.
- **Target repos must declare `language.version` ≤ the pinned engine (v0.15.4).** `../infra` is
  already v0.15.4; init's scaffold (B3b) writes v0.15.4. Bump the pin (and this) when defs
  supports a newer CUE.
- Adding `cuelang.org/go` pulled it as a direct dep (+ bumped cobra/pflag transitively).
- **Don't reintroduce a `cue` binary shell** for render or mod ops — use the linked library.
</content>
