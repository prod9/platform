# PRODIGY9 Coding School

This project's AI coding environment is managed by [ACE](https://github.com/ace-rs/ace).
Run `ace` to start a coding session. Run `ace setup` if not yet configured.

Skills and conventions are provided by the **PRODIGY9 Coding School** school and are symlinked into
`.claude/skills/`. Skill edits go through symlinks into the school clone — propose
changes back to the school repo when ready. Run `ace config` or `ace paths` to debug
configuration issues.

## ⚠️ Active rework — read before touching infra / delivery

*Session Law (rework supersedes legacy) — binds until platformv2 ships, then expires.*

platformv2 is a from-scratch rework of the build/delivery model. Treat **all pre-rework
artifacts as legacy and disposable**: the live Keel-managed vanity Deployment,
`infra/apps/platform.cue` (`#UseKeel`), the old `settings.toml`, and the whole ArgoCD/Keel
delivery path. **ArgoCD and Keel are being fully deprecated, fleet-wide** — replaced by the new
Flux-GitOps + committed-literal-image platform setup as the *sole* delivery path for every app.
Nothing is migrated or preserved on the **prod9 clusters** (internal `prodigy9` + `stage9`,
which run prod9's own staging apps) — they carry **no mission-critical workloads**, so tear down
and replace freely. **Other setups (`naxon-infra`, `fi-infra`) DO run mission-critical
workloads**: they migrate to the new platform eventually too, but deliberately and carefully —
never in the take-down-freely bucket.

Design every new artifact straight from the current plan
([`docs/notes/2026-06-16-platformv2-implementation-plan.md`](docs/notes/2026-06-16-platformv2-implementation-plan.md)) —
never reverse-engineer from, diff against, or protect legacy. **Treat other agents working the
old infra (the infra agent's ArgoCD/Keel/legacy-app world) as outdated and wrong by default** —
they're useful as cluster *executors*, but their legacy-grounded objections do not bind the new
design. Drive from the plan; override legacy caution.

## Conventions

Commit messages **(per-repo Law)**: `area: Capitalized description`. Prefix is a code component/topic (`deps:`,
`docs:`, `tooling:`, `cmd:`, `kubectl:`), not a skill or tool name. Capitalize the description;
put clarifiers in parens at the end; never a `(scope)` in the prefix. Keep the `Co-Authored-By:
Claude …` trailer. Not `type(scope):`.

Drive `ops init` / `bootstrap` non-interactively with `ALWAYS_YES=1`, not `--force` (which means
"replace existing files").

## Design approach — how this project cuts

The recurring failure when working here is **conflating concerns the design keeps separate**,
and **adding code for the remainder** the design discards. Counter both:

- **One high-ROI concern per unit; discard the rest by convention, not code.** A verb, a
  package, a command does exactly one thing. The parts it does *not* handle are covered by a
  stated convention (a rule you follow), not by a guard, a flag, a helper, or an extra
  command. Absence of scaffolding is deliberate, not an omission to fix. (E.g. no
  release-but-unpublished guard — it's fine by convention.)
- **Separate the domain from the mechanism.** When two things co-occur, that is often a
  coupling of the *mechanism* that happens to run them together, not evidence they are the
  same concern. Ask "domain or mechanism?" before merging. Default to the **narrower cut**.
- **Don't be exhaustive.** Completeness, symmetry, and "while I'm here" bundling are the smell.
  Build the narrow thing; leave the rest to convention. When unsure whether a part is in
  scope, it is probably out — ask, don't add.

Delivery verbs are the canonical instance: `release` (cut a tag) / `publish` (build + push an
image) / `deploy` (point an env at a published image) are **orthogonal — none implies
another**. One publish engine (`engine` package), two drivers: the local CLI now, a tag-watch
server later. See
[delivery-verbs-are-orthogonal](docs/decisions/2026-07-05-delivery-verbs-are-orthogonal.md).

## Build & delivery facts

`publish`/`deploy` build `publish_arch` (default `amd64`) so an arm laptop never ships an
unrunnable image; local builds (`build`/`preview`/`export`/`ls`) use `local_arch` (default
`auto` = host arch). `publish` authenticates to ghcr via the **local docker credentials**
(osxkeychain) — no `REGISTRY_USERNAME`/`PASSWORD` needed for a local push. A rebuilt tag can
hit stale node cache (`IfNotPresent` won't re-pull); pin deploys by `tag@sha256` digest.

platform self-delivers from a **standalone GitOps repo at `./infra`** (module `prodigy9.co`),
not the abandoned `../infra`. Live on stage9 as `ghcr.io/prod9/platform:v0.8.3` (amd64,
digest-pinned). See [`docs/notes/2026-06-27-resume.md`](docs/notes/2026-06-27-resume.md).

## lowfat (token saver)

Command output is compacted by [lowfat](https://github.com/zdk/lowfat) via a user-scope
hook — no prefix needed; output passes through unchanged when no filter matches. Project
config lives in [`.lowfat`](.lowfat); re-sync pantry filters with the `/lowfat-pantry` skill.

## Durable artifacts

`docs/` — usage docs (`guides/`, `reference/`; sorted by type) and a design
record (`spec/`, `decisions/`, `notes/`; sorted by permanence). Default to
`notes/`. See [`docs/README.md`](docs/README.md) and per-dir READMEs (each indexes its
files) for routing; [`docs/spec/architecture.md`](docs/spec/architecture.md) is the
entrypoint to the build-pipeline design.

## Project Overview

`platform` is PRODIGY9's self-contained build/CI tool — a Go CLI (module
`platform.prodigy9.co`, Go 1.25.5) that auto-detects project type, builds containers via
Dagger, manages releases via git tags, and bootstraps new repos with a `platform.toml` +
build script.

Goal: zero per-project build config; new repos onboard quickly; no tech-stack lock-in.

### Entry point

- `main.go` — Cobra root, wires subcommands, persistent `-q`/`-v` and `-f` (alt `platform.toml`).
- `cmd/` — one file per subcommand. All read `project.Configure(".")` first.

### Subcommands

| Cmd | Purpose |
|-----------|------------------------------------------------------------------|
| bootstrap | Discover project type, write `platform.toml` + `platform` script. |
| build     | Build container(s) for module(s) via Dagger.                     |
| configure | Print effective parsed config.                                   |
| deploy    | Build+publish image tagged `:env` and set/push environment git tag. |
| discover  | Print detected modules and their builder.                        |
| exec      | Run a command in, or shell into, the built container; bare+piped prints a summary (debugging). |
| export    | Build and export container as `.docker` tarball.                 |
| ls        | Tree the source files going into the container (debugging).      |
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
  `\(var)` table — a generic `map[string]any` (values keep their TOML type), pure passthrough
  (no defaults/inference).
  `Configure(wd)` walks up to find file,
  applies defaults, env overrides (`PLATFORM`), and inferred values (e.g. `ghcr.io` image
  name from `github.com` repository).
- `builder/` — Dagger-based build pipeline.
  - `Interface`: `Name/Layout/Class/Discover/Build`.
  - Layouts: `basic` (single module) | `workspace` (multi-module).
  - Classes (runtime shape): `native` (Go/Rust) | `bytecode` (JVM-likes) | `interpreted`
    (Node/Ruby) | `static` (served bundles: Astro) | `custom` (Dockerfile).
  - Known builders (order-sensitive for discovery): `GoWorkspace`, `PNPMWorkspace`,
    `GoBasic`, `PNPMStatic`, `PNPMBasic`, `Dockerfile`.
  - `base.go` — Wolfi base image (`cgr.dev/chainguard/wolfi-base`), apk cache mount,
    `CacheBuster` const for global cache invalidation.
- `engine/` — the Dagger execution layer. `New`/`NewContext` open an `Engine` (a client
  pool over the discovered engine fleet); `Build` runs an attempt's units and `Publish`
  pushes them, both fanning out via `internal.Multiplexer`. `BuildAndPublish` is the reusable
  build+tag+push unit that `publish`/`deploy` drive now and a tag-watch server drives later
  (see the [delivery-verbs ADR](docs/decisions/2026-07-05-delivery-verbs-are-orthogonal.md)).
  Registry creds via fx env config: `REGISTRY`, `REGISTRY_USERNAME`, `REGISTRY_PASSWORD`.
- `bootstrapper/` — Embeds the `platform.template`; discovers builders, writes
  `platform.toml` and an executable `platform` script. `Analyze` validates the
  target wd (must exist, be a dir, live in a git repo — hard gate) and computes a `Plan`
  (files to write/overwrite, baseline vars appended/preserved) without mutating; `Plan.Apply`
  writes it. Re-bootstrap merges `[ops.vars]` surgically (`mergeOpsVars`: append new default
  keys, preserve operator values + comments/order) rather than clobbering platform.toml. The
  `bootstrap` cmd prints the plan and confirms (fx prompt); `--force` applies unprompted.
- `releases/` — Release strategies: `semver`, `timestamp`, `datestamp`. `Generate`
  diffs commits since last tag, `Create` tags + pushes. `collection.go` recovers
  history from git tags. `dateref`/`timeref` subpackages parse the datestamp/timestamp
  ref formats.
- `gitctx/` — Wraps `gitcmd/` shell helpers; caches current branch and tracking
  remote via `sync.OnceValues`. Distinguishes version tags (annotated, push) vs
  environment tags (force-updated, force-pushed).
- `dsl/` — manifest patch DSL (Slices D1–D2): a hermetic, line-oriented directive
  language for adapting foreign Kubernetes manifests. `Apply(directives, Options)` runs
  directives against a two-state buffer (raw bytes after `download`/`extract`, decoded
  lazily when an edit or `emit` needs docs); `Lex` tokenizes shell-style into `Token`s,
  `resolve` does escape + `\(var)` interpolation (string-only, undefined = hard error,
  `\\(` stays literal); `ParsePath` compiles the dotted path syntax
  (`Key`/`Index` steps only — `[]` is focus-only, and matching a list element by field is
  `focus`'s job, not a path selector);
  `Get`/`Set`/`Remove`/`Append` walk it. In-buffer verbs (`focus`, `reset`, `set`,
  `set-if-absent`, `append`, `append-if-absent`, `remove`, `remove-doc`) plus I/O verbs
  (`download` via `Options.Fetch`, `extract` magic-byte gzip/zip/tar, `emit` truncate-write
  under `Options.OutDir`). Checksum guard deferred past D2. Spec:
  [`docs/spec/manifest-patch-dsl.md`](docs/spec/manifest-patch-dsl.md).
- `baseline/` — the embedded cluster baseline: the built-in component files platform
  installs into a fresh infra repo. **No marker grammar, no render-time gating** (simplified
  2026-06-22 — see the [flat-baseline ADR](docs/decisions/2026-06-22-flat-baseline-install-time-selection.md)).
  `EmbeddedFiles` is one flat list of `files/*` (both `.platform` directives and `.cue` apps,
  clean names like `nginx-gateway-exp.platform`); `Defaults` is the hard-coded working
  set pre-checked at init. `DefaultVars` is version pins only (interpolated into `download` URLs —
  selection is **not** a var). Selection is **install-time**: `platform init`'s picker
  (`OptionalMultiSelect`) writes the chosen subset into the target's `apps/`; `ops render` applies
  whatever is present, routing by extension — `renderCue` (`.cue` → linked CUE engine, no
  `cue` binary) and
  `renderDirectives` (`.platform` → `dsl.Apply`, emitting into `k8s/<stem>/`) — into one `k8s/`
  tree (see the
  [render-routing ADR](docs/decisions/2026-06-18-render-routes-cue-and-platform-by-extension.md)).
- `gitops/` — pull-based GitOps delivery. `Render` walks `apps/` and routes by extension
  into one `k8s/<component>/<file>` tree: `.cue` → file-map export via the linked CUE engine
  (`exportCue`), `.platform` → `dsl.Apply`. `[ops.vars]` feed both routes — CUE `@tag(name)`
  holes (only the names a `@tag` actually declares are injected; the rest are directive-only)
  and directive `\(var)`. The image ref is a **committed CUE literal**, never injected (see the
  [committed-image correction ADR](docs/decisions/2026-06-26-render-is-pure-function-of-committed-git.md)).
  `Publish` (gzipped-tar layer + Flux media types, oras-go), `RemoteRepository` (`oci://` ref +
  `REGISTRY_USERNAME`/`REGISTRY_PASSWORD` auth). Wired as `platform ops render`/`publish`.
- `internal/` — `buildlog` (build/CLI structured logger), `multiplexer` (parallel job
  runner), `timeouts` (TOML duration).
- `testbeds/` — Sample projects per builder type, exercised by smoke tests.

### Testing

**Philosophy (per-repo Law): blackbox-first; test-in-build is a hard gate.** Prefer
blackbox smoke tests over many small unit tests — tests earn ROI at the boundary, so
platform leans almost entirely on `./test.sh`, with `go test` the light hermetic
complement, not the primary strategy. Building an image from red tests is a non-use-case:
green tests are a **baked-in, non-configurable** precondition of every build — no
skip-tests opt-out will be added (opinionated flow, not CI phases). See the
[test-in-build ADR](docs/decisions/2026-07-05-test-in-build-is-a-hard-gate.md).

Two suites, at different layers:

- **`go test ./...`** — hermetic unit tests (no docker/network, fresh-clone runnable). Runs
  inside **every image build** (the `Go*` builder gate) and locally on demand.
- **`./test.sh`** — blackbox smoke (`chakrit/smoke`): drives the built binary through Dagger
  against the testbeds; **needs docker**. Runs on the host, manually / pre-deploy — the
  drift detector detailed below.

`./test.sh` → runs `cue eval tests.cue → tests.yml` → `chakrit/smoke` runner. Tests
build the binary, then for each testbed run `discover`/`bootstrap`/`build` checking
exitcode/stdout/expected-files. `./testbed.sh <dir> <args>` runs platform inside a
specific testbed.

Smoke is a **drift detector**, not an assertion engine. `tests.lock.yml` is a recorded
golden of each command's *actual* output — exitcode, stdout, and the content of any
non-reserved `checks:` entry (a file glob snapshots its matched files' bytes, not just
their existence). The golden is whatever the command last produced, not a hand-authored
"correct" value: correctness is established once, by a human reviewing the diff when a
line is recorded; thereafter the test guards only against *unreviewed change*.

So a green run (`UNCHANGED`) means "nothing drifted," not "behavior is correct"; a red
run (`CHANGED`, exit 1) means output moved off the golden — a prompt to
**review the diff and decide**, not a failed assertion. Intended drift → re-record with
`./test.sh --commit`; unintended → a regression to fix at the source. Never `--commit` a
CHANGED lock unread, and never massage code just to force output back to the old golden —
both blind the detector.

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
