# PRODIGY9 Coding School

This project's AI coding environment is managed by [ACE](https://github.com/ace-rs/ace).
Run `ace` to start a coding session. Run `ace setup` if not yet configured.

Skills and conventions are provided by the **PRODIGY9 Coding School** school and are symlinked into
`.claude/skills/`. Skill edits go through symlinks into the school clone — propose
changes back to the school repo when ready. Run `ace config` or `ace paths` to debug
configuration issues.

## lowfat (token saver)

Command output is compacted by [lowfat](https://github.com/zdk/lowfat) via a user-scope
hook — no prefix needed; output passes through unchanged when no filter matches. Project
config lives in [`.lowfat`](.lowfat); re-sync pantry filters with the `/lowfat-pantry` skill.

## Durable artifacts

`docs/` — usage docs (`guides/`, `reference/`; sorted by type) and a design
record (`spec/`, `decisions/`, `notes/`; sorted by permanence). Default to
`notes/`. See [`docs/README.md`](docs/README.md) and per-dir READMEs for routing.

## Project Overview

`platform` is PRODIGY9's self-contained build/CI tool — a Go CLI (module
`platform.prodigy9.co`, Go 1.25.5) that auto-detects project type, builds containers via
Dagger, manages releases via git tags, and bootstraps new repos into Buildkite CI.

Goal: zero per-project build config; new repos onboard quickly; no tech-stack lock-in.

### Entry point

- `main.go` — Cobra root, wires subcommands, persistent `-q`/`-v` and `-f` (alt `platform.toml`).
- `cmd/` — one file per subcommand. All read `project.Configure(".")` first.

### Subcommands

| Cmd | Purpose |
|-----------|------------------------------------------------------------------|
| bootstrap | Discover project type, write `platform.toml` + `platform` script + `.buildkite/pipeline.yaml`. |
| build     | Build container(s) for module(s) via Dagger.                     |
| configure | Print effective parsed config.                                   |
| deploy    | Build+publish image tagged `:env` and set/push environment git tag. |
| discover  | Print detected modules and their builder.                        |
| export    | Build and export container as `.docker` tarball.                 |
| ls        | List files inside built container (debugging).                   |
| ops       | GitOps delivery namespace: `ops render` (infra CUE → manifests), `ops publish` (push as OCI config artifact). |
| preview   | Build and serve container locally via Dagger tunnel.             |
| publish   | Build+publish image tagged `:release-name` from latest release tag. |
| release   | Create new release tag (semver/timestamp/datestamp); supports `-p/-m/--major`. |
| vanity    | Hidden HTTP server: redirects `go get platform.prodigy9.co` to GitHub. |

### Packages

- `project/` — `platform.toml` parser. `Project` (maintainer, repository, strategy,
  environments, excludes, modules, `[ops]`) and `Module` (workdir, builder, env, port,
  cmd, args, asset_dirs, build_dir, image, package). `[ops]` (`Ops.Image`/`Tag`) is the
  `ops publish` target — inferred from `repository` (`ghcr.io/x`) with `tag` defaulting to
  `latest`; `Ops.Ref(tag)` resolves the ref. `Ops.Vars` (`[ops.vars]`) is the verbatim DSL
  `\(var)` table — a generic `map[string]string`, pure passthrough (no defaults/inference).
  `Configure(wd)` walks up to find file,
  applies defaults, env overrides (`PLATFORM`), and inferred values (e.g. `ghcr.io` image
  name from `github.com` repository).
- `builder/` — Dagger-based build pipeline.
  - `Interface`: `Name/Layout/Class/Discover/Build`.
  - Layouts: `basic` (single module) | `workspace` (multi-module).
  - Classes: `native` (Go/Rust) | `bytecode` (JVM-likes) | `interpreted` (Node/Ruby) |
    `custom` (Dockerfile).
  - Known builders (order-sensitive for discovery): `GoWorkspace`, `PNPMWorkspace`,
    `GoBasic`, `PNPMStatic`, `PNPMBasic`, `Dockerfile`.
  - `base.go` — Wolfi base image (`cgr.dev/chainguard/wolfi-base`), apk cache mount,
    `CacheBuster` const for global cache invalidation.
  - `Build/Publish` use `internal.Multiplexer` for parallel job execution.
  - Registry creds via fx env config: `REGISTRY`, `REGISTRY_USERNAME`, `REGISTRY_PASSWORD`.
- `bootstrapper/` — Embeds templates (`platform.template`,
  `buildkite.pipeline.yaml.template`); discovers builders, writes `platform.toml`,
  executable `platform` script, and `.buildkite/pipeline.yaml`. `Analyze` validates the
  target wd (must exist, be a dir, live in a git repo — hard gate) and computes a `Plan`
  (files to write/overwrite, baseline vars appended/preserved) without mutating; `Plan.Apply`
  writes it. Re-bootstrap merges `[ops.vars]` surgically (`mergeOpsVars`: append new default
  keys, preserve operator values + comments/order) rather than clobbering platform.toml. The
  `bootstrap` cmd prints the plan and confirms (fx prompt); `--force` applies unprompted.
- `releases/` — Release strategies: `semver`, `timestamp`, `datestamp`. `Generate`
  diffs commits since last tag, `Create` tags + pushes. `collection.go` recovers
  history from git tags.
- `gitctx/` — Wraps `gitcmd/` shell helpers; caches current branch and tracking
  remote via `sync.OnceValues`. Distinguishes version tags (annotated, push) vs
  environment tags (force-updated, force-pushed).
- `core/dsl/` — manifest patch DSL (Slices D1–D2): a hermetic, line-oriented directive
  language for adapting foreign Kubernetes manifests. `Apply(directives, Options)` runs
  directives against a two-state buffer (raw bytes after `download`/`extract`, decoded
  lazily when an edit or `emit` needs docs); `Lex` tokenizes shell-style into `Token`s,
  `resolve` does escape + `\(var)` interpolation (string-only, undefined = hard error,
  `\\(` stays literal); `ParsePath` compiles the dotted path syntax
  (`Key`/`Index`/`Select` steps, incl. `[field=val]` field-select);
  `Get`/`Set`/`Remove`/`Append` walk it. In-buffer verbs (`select`, `reset`, `set`,
  `set-if-absent`, `append`, `append-if-absent`, `remove`, `remove-doc`) plus I/O verbs
  (`download` via `Options.Fetch`, `extract` magic-byte gzip/zip/tar, `emit` truncate-write
  under `Options.OutDir`). Checksum guard deferred past D2. Spec:
  [`docs/spec/manifest-patch-dsl.md`](docs/spec/manifest-patch-dsl.md).
- `core/baseline/` — the DSL assembly layer (Slice D3b-2): turns the embedded cluster
  baseline's `.platform` (platform-DSL) directive files into the set to apply, gated by
  `[ops.vars]` at **whole-file** granularity (DSL stays branch-free). Filename convention:
  `name.platform` always applies, `name@variant.platform` is one variant of choice group `name`
  (applied when `vars[name] == variant`), `name+flag.platform` is an overlay (applied when
  `vars[flag] == "true"`). `ScanOptions` surfaces operator-selectable knobs (for bootstrap
  prompts), `Select` resolves the file set (unknown choice value = hard error). Running these
  directives (fetch upstream + patch + emit) is a **separate activity from `ops render`**
  (`cue export`): foreign-install adaptation and authored-manifest rendering don't mix.
- `core/gitops/` — pull-based GitOps delivery (Slice 1). `Render` (`cue export -e objects`
  → multi-doc YAML), `Publish` (gzipped-tar layer + Flux media types, pushed via oras-go),
  `RemoteRepository` (`oci://` ref + `REGISTRY_USERNAME`/`REGISTRY_PASSWORD` auth). Wired
  as `platform ops render`/`publish`.
- `internal/` — `plog` (structured logger), `multiplexer` (parallel job runner),
  `timeouts` (TOML duration), `fileutil`, `dateref`, `timeref`.
- `testbeds/` — Sample projects per builder type, exercised by smoke tests.

### Testing

`./test.sh` → runs `cue eval tests.cue → tests.yml` → `chakrit/smoke` runner. Tests
build the binary, then for each testbed run `discover`/`bootstrap`/`build` checking
exitcode/stdout/expected-files. `./testbed.sh <dir> <args>` runs platform inside a
specific testbed.

The 1m per-test timeout in `tests.cue` is deliberately tight — it keeps builds honest.
Never raise it to make a slow build pass: fix the slowness (cache reuse, unnecessary
work, network pulls) instead, since a slowdown landed by one person taxes everyone's
local and CI cycles. Cold-cache pulls of a freshly pinned image are the one accepted
cause — verify by warming the cache and re-running, not by touching the timeout.

### Key dependencies

`dagger.io/dagger` (container builds), `fx.prodigy9.co` (config + cmd prompts +
ctrlc), `BurntSushi/toml`, `spf13/cobra`, `pterm/pterm`, `go.jonnrb.io/vanity`.

## Load these skills

Default skill set for this project (consumed by `ace.toml`):

- `ace*` — session workflow, save, audit, realign, school
- `general-coding` — per-slice workflow + cross-language conventions
- `go-coding` — Go is the implementation language
- `prod9-fx` — `fx.prodigy9.co v0.4.0` is in `go.mod`
- `cue-coding` — `tests.cue` drives the smoke harness
- `shell` — `test.sh`, `testbed.sh`, embedded `platform` script template
- `markdown-writing` — for editing this file
