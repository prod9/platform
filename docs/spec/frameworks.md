# Frameworks

Status: **design-of-record.** A `Framework` is the **sole owner of a project type** — it
recognizes itself (`Discover`), scaffolds itself (`Scaffold`), and knows how it is built
(`Plan` + `Execute`).
This spec owns the per-stack strategies, stack discovery, and the shared Wolfi base; it
sits at the `interpret`/`strategies` stages of the pipeline. The
[architecture spec](architecture.md) frames the pipeline and the two data models,
[engine](engine.md) owns execution, and [scaffolding](scaffolding.md) owns the scaffold
mechanism and `cmd/init` orchestration — read [architecture.md](architecture.md) first.

## The `Framework` contract

A framework is a stateless value (an empty struct) implementing `framework.Framework`. It
carries per-stack knowledge and nothing else — no config, no engine handle, no build
state. Seven methods:

| Method                                            | Returns   | Role                                                      |
| ------------------------------------------------- | --------- | --------------------------------------------------------- |
| `Name() string`                                   | id        | Stable id (`go/basic`, `pnpm/static`, …); `[modules]` key |
| `Layout() Layout`                                 | shape     | `basic` \| `workspace` — module topology                  |
| `Discover(wd string) bool`                        | detect    | True if this stack owns `wd` (scaffold-time only)         |
| `RequiredScaffoldInputs(wd) []string`             | inputs    | Operator inputs to prompt at init, by name (usually nil)  |
| `Scaffold(ctx, wd, env, inputs) Spec`             | seed      | The framework's full, **resolved** contribution (below)   |
| `Plan(*BuildUnit) []Step`                         | steps     | The ordered steps this unit's build is made of            |
| `Execute(ctx, client, *BuildUnit, Step, in) (out, error)` | container | Run **one** step: container in → container out    |

`Discover` and `Scaffold` are scaffold-time: the build path reads `[modules]` (which pins
`Name`), it never re-discovers.

`Plan` and `Execute` are handed the raw `*dagger.Client` (on `Execute`) and a
fully-resolved `BuildUnit` (workdir, arch, env, command, asset dirs, image name, vars) —
never the engine. Per architecture's consumer-defined-interface rule the strategy needs
only a client, which is what keeps `framework` from importing `engine`.

## Steps — the framework is dumb, one step at a time

A framework does **not** own the build loop. It answers two questions: *what steps does
this unit take* and *how do I run one of them*. The engine drives.

```go
type Step string   // opaque label; Stringer
```

`Step` is an opaque label, nothing more — no payload, no closure, no `map[Step]opFn`. A
framework's `Plan` returns its ordered steps; `Execute` switches on the `Step` it is
given, does that stage's work, and returns the container the next step should receive. The
first step receives a nil container (it establishes the base); each subsequent step
receives its predecessor's output.

**The container is the chaining medium, not the contract.** A step is not obliged to
transform what it is handed: `Infra`'s `deps` step fetches CUE dependencies on the *host*
and passes its input straight through, which is a legitimate step, not a degenerate one.
What every step owes is a container for its successor — nothing about where its work
happens. Nothing is stored between calls — the framework holds
no build state, so a plan can be recomputed at any time and the engine always drives the
latest one.

🚨 **Step granularity IS log granularity.** The engine times every step and flushes that
step's captured output at its boundary, so a long step is a long silence with no remedy
short of cutting it into smaller steps. "Every time-taking step is visible and timed" is
therefore a **constraint on how `Plan` is authored**, not a UX preference: exec-heavy work
(dependency fetch, test, compile, image assembly) each earns its own `Step`. Steps with no
exec at all — the `Infra` framework's pure `WithDirectory` tree write — are still timed,
they are simply silent.

`Scaffold` is **rich, per-framework** — not an empty seam. It returns the framework's full
contribution to a fresh repo (`scaffold.Spec`): its `platform.toml` module, the default
`[vars]` it seeds, the files it ships, and the default `strategy` value it seeds. The
framework owns resolution — which operator input fills which template hole, reading an
existing `cue.mod` — so the files come back **resolved** and `cmd/init` gathers the inputs
`RequiredScaffoldInputs` declares, generates `platform.toml`, and writes finished bytes.
There is **no `IsInfra` / app-vs-infra predicate**: `Infra.Scaffold` simply *does more*
(it contributes the whole cluster baseline and a `strategy="rolling"` seed), so the
app/infra distinction is pure `Scaffold` polymorphism. The `scaffold.Spec`/`scaffold.File`
shapes and the resolve mechanism live in [scaffolding](scaffolding.md).

## Layouts

The module's topology on disk. Selects how `Build` roots the Dagger host directory.

| Layout      | Meaning                                              | Marker                            |
| ----------- | ---------------------------------------------------- | --------------------------------- |
| `basic`     | Single self-contained module; `WorkDir` is the root  | `go.mod`, `pnpm-lock.yaml`, …     |
| `workspace` | Module is one member of a multi-module workspace     | `go.work`, `pnpm-workspace.yaml`  |

Workspace frameworks root the host directory one level **up** from the module
(`filepath.Join(unit.WorkDir, "..")`) so the workspace file and sibling modules come into
the build, then select the target module by name.

## Runtime shape families

A **descriptive taxonomy, not a contract method** — the family names what has to be present
in the runtime container to run the artifact. Orthogonal to the build language; it
describes what a framework's runner stage lays down.

| Family        | Produces                            | Runtime needs                  | Examples          |
| ------------- | ----------------------------------- | ------------------------------ | ----------------- |
| `native`      | Machine-native binary               | Nothing but the binary         | Go, Rust          |
| `bytecode`    | Non-native binary                   | A VM/runtime                   | Java, Erlang, Elixir |
| `interpreted` | Bundled/packaged sources (no build artifact) | Same toolchain as buildtime | Node, Rails       |
| `static`      | Static asset bundle                 | A webserver only, no runtime   | Astro, Hugo, HTML |
| `custom`      | Anything; escapes the taxonomy      | Whatever the build defines     | Dockerfile, Infra |

`native` copies just the compiled binary into a lean runner. `interpreted` carries build
output plus `node_modules`. `static` drops in a Caddy file-server and the built bundle
with no language runtime. `custom` owns its own base and runtime entirely (Dockerfile uses
the user's `FROM`; `Infra` packs a `FROM scratch` manifest image).

## Discovery — first match wins

The package-level `framework.Discover(wd)` resolver walks the known frameworks in order and
returns the **first** whose `Discover` is true. `FindFramework(name)` resolves a framework
by `Name` for the build path (reads the `[modules]` `framework` key; the legacy `builder`
key is a deprecated read-alias — scaffolding writes only `framework`). The list is
**order-sensitive** — several stacks' markers coexist in one tree, so the broader/more-specific
match must be checked before the one it would also satisfy.

Order and detection rules:

| # | Framework       | Name             | Layout      | Family        | Detects on                                    |
| - | --------------- | ---------------- | ----------- | ------------- | --------------------------------------------- |
| 1 | `Infra`         | `platform/infra` | `basic`     | `custom`      | Dir name contains `infra` (glob, not a file)  |
| 2 | `GoWorkspace`   | `go/workspace`   | `workspace` | `native`      | `go.work`                                     |
| 3 | `PNPMWorkspace` | `pnpm/workspace` | `workspace` | `interpreted` | `packages:` key in `pnpm-workspace.yaml`      |
| 4 | `GoBasic`       | `go/basic`       | `basic`     | `native`      | `go.mod`                                      |
| 5 | `PNPMStatic`    | `pnpm/static`    | `basic`     | `static`      | `astro.config.mjs`                            |
| 6 | `PNPMBasic`     | `pnpm/basic`     | `basic`     | `interpreted` | `pnpm-lock.yaml`                              |
| 7 | `Dockerfile`    | `dockerfile`     | `basic`     | `custom`      | `Dockerfile`                                  |

Why the order holds:

- **Infra first** — the `Infra` framework's `Discover` heuristic is a directory *name*
  glob, not a file marker; `apps/` is a poor marker (an ordinary app may also carry
  `apps/`). Checked ahead of file markers so an infra repo never mis-detects on a stray
  lockfile.
- **Workspace before basic** (2 before 4, 3 before 6) — a Go workspace repo also holds
  `go.mod` files; a pnpm workspace also holds a `pnpm-lock.yaml`. The workspace marker is
  the broader truth, so it must win before the basic marker it would also trip.
- **A pnpm workspace is the `packages:` key, never the file.** From pnpm v10 every repo
  carries `pnpm-workspace.yaml` — it is where all non-auth settings live, including the
  `allowBuilds` approvals a single-package repo must commit. Presence therefore says
  nothing; `packages:` is what pnpm itself defines a workspace by, so discovery reads the
  file and keys on that. Detecting on existence makes every modern pnpm repo a workspace.
- **Static before basic** (5 before 6) — an Astro project carries `pnpm-lock.yaml` too;
  the `astro.config.mjs` signal is the more specific one and must be checked first, else
  every Astro repo detects as `pnpm/basic`.
- **Dockerfile last** — the escape hatch. It bypasses the Wolfi base and package
  conventions and emits a runtime warning; every language-specific framework is preferred,
  so it only wins when nothing else matched.

## The shared base

Every framework except `Dockerfile` (own `FROM`) and `Infra` (`FROM scratch`) starts from
`BaseImageForUnit` — Chainguard's Wolfi base (`cgr.dev/chainguard/wolfi-base`), small,
regularly patched, shared across all language stacks.

**Wolfi is glibc-based** (`glibc` is installed in the bare base; there is no musl). That is
the answer to every "will this prebuilt binary run?" question here — stock Linux binaries
and CGO builds work unmodified, and nothing needs a musl variant. Do not describe this base
as musl or "glibc-free"; it is neither.

- **Never pinned.** `BaseImageName` is `cgr.dev/chainguard/wolfi-base:latest` — a floating
  tag, resolved at build time, never a digest. Wolfi is rolling and the repository carries
  exactly one real tag (`latest`), so there is no version to track even if we wanted one.
  Userland is refreshed every build via `apk update && apk upgrade`.
- **apk cache mount.** `/var/cache/apk` mounts the persistent `platform-apk-cache` volume
  so package pulls survive across builds.
- **`CacheBuster`.** A const written into the image (`/<CacheBuster>`) to force Dagger and
  Docker to invalidate cached base layers across all environments. Bump it to shed a stale
  base layer.

The base lays down a fixed FHS-style tree so an operator shelling in always finds things
in the same place: `SrcDir` (`/platform/src`, build workspace), `BinDir` (`/platform/bin`,
on `PATH`), `RunDir` (`/platform/run`, runtime workdir). Package sets are applied via
`withBuildPkgs` (`build-base git curl bash` + extras) for the build stage and
`withRunnerPkgs` (`ca-certificates curl mailcap netcat-openbsd tzdata` + extras) for the
runner; `withCaddyServer` adds Caddy for the static family. `mailcap` is carried only to
lay down `/etc/mime.types` — the Wolfi base ships neither that file nor `/etc/mailcap`, and
a server without the table falls back to a built-in list that misses modern types (`.woff2`
among them), so assets go out with the wrong `Content-Type`. It belongs to every runner,
not just the static family.

🚨 **The base is Wolfi, not Alpine — never resolve a package against Alpine's index.** Both
use `apk` and the names often coincide, which is exactly what makes the wrong lookup pass
review. How to resolve one properly: [`../vendor/wolfi.md`](../vendor/wolfi.md).

## Test-in-build is a hard gate

The Go frameworks run the module's tests **inside the image build**, as their own `Step`
ahead of the compile step: `GoBasic` execs `go test -v ./...`, `GoWorkspace` execs
`go test -v` across every workspace module. Because Dagger fails the build on a non-zero
exec, that step fails the run — **green tests are a
baked-in, non-configurable precondition of a Go image** — a red suite is a failed build,
and there is no skip-tests opt-out. Full rationale:
[test-in-build-is-a-hard-gate](../decisions/2026-07-05-test-in-build-is-a-hard-gate.md).

## Stack notes

- **Go** — pins the exact toolchain from `go.mod`/`go.work` via native `GOTOOLCHAIN`
  (`withGoVersion`); mounts per-version module and build caches (`withGoCaches`). go.mod/
  go.sum (and every member's in workspace) are copied and `go mod download`-ed before the
  full source, so the dependency layer keys on manifests alone. Runner carries only the
  compiled binary.
- **pnpm** — Node comes from nodejs.org via `tj/n` (`n install lts`), pnpm via Node's
  corepack. `pnpm/basic` and `pnpm/workspace` serve via bare `node`; `pnpm/static` serves
  the built bundle with Caddy `file-server`. Workspace runner marks `RunDir` as ESM
  (`withPNPMModuleFix`).

  🚨 **Platform names no toolchain version, anywhere — never pin.** Node is whatever `n`
  calls `lts`; pnpm is whatever the repo's `package.json` `packageManager` field says, which
  corepack resolves per-project. Platform declares no Node or pnpm version of its own, and a
  repo without `packageManager` is simply not built — platform is opinionated about how each
  stack builds, and the way is to always declare it. This is the same rule as the Wolfi base
  above: nothing in platform is version-pinned.

  🚨 **This provisioning is deliberate — never "simplify" it to distro packages.** Never
  `apk add nodejs`/`corepack`. Node, corepack, pnpm, and the distro are four uncoordinated
  maintainer groups; sourcing Node from the distro adds a party whose repackaging borks
  the seams downstream (linux-wifi-driver style). Stay closest to the least-magic,
  most-reliable upstream. pnpm over npm because npm is slow; corepack because it is
  Node-sanctioned and narrow-jobbed, not a version-juggler like nvm. A cache or build
  failure in this step is **never** a reason to switch to apk — shed the cache with
  `platform clean` (first-line diagnostics for any "worked on a fresh checkout but not
  here" failure) and fix the real cause.

  🚨 **The dependency layer carries `pnpm-workspace.yaml`, not just the two manifests.**
  From pnpm v10 that file is the *only* place non-auth settings live — `.npmrc` keeps auth
  and registry alone — and it is where `pnpm approve-builds` records which dependencies may
  run install scripts (`allowBuilds`; `onlyBuiltDependencies` pre-v11). A dep layer copying
  only `package.json` + `pnpm-lock.yaml` therefore drops approvals the repo has already
  committed, and every dependency needing a build script silently goes unbuilt. Build
  approval is **repo state**: the repo runs `pnpm approve-builds` and commits the result,
  and platform's job is only to stop hiding the file — no platform config key, no
  `--dangerously-allow-all-builds`, which would diverge from what the developer gets
  locally. The basic and static frameworks select the manifests by include-filtering the
  host directory, so an absent `pnpm-workspace.yaml` is simply not copied; `pnpm/workspace`
  copies the whole tree and already carries it.
- **Dockerfile** — `host.DockerBuild` on the user's `Dockerfile`; env becomes build args.
  Discouraged: bypasses Wolfi, the apk cache, and package conventions; warns at build
  time.
- **Infra** — its render step calls `gitops.Render` in-process (CUE + `.platform` →
  manifest tree), then writes the whole tree into a `client.Container()` with no `From`
  (scratch) in **one** `WithDirectory` — a per-file `WithNewFile` walk yields a
  multi-layer image and Flux's source-controller extracts only one layer, so `prune` then
  wipes the cluster. The published layer is a tar+gzip of exactly those files, which
  Flux's `OCIRepository` `layerSelector` extracts; kustomize-controller applies the YAML.
  Infra delivery is the ordinary `publish` verb — see
  [infra-publishes-as-plain-image-retire-oras](../decisions/2026-07-05-infra-publishes-as-plain-image-retire-oras.md).

## Frameworks and the build model

**The way to get a build running is an `engine` entrypoint — `cfg` + module names in,
results out.** Reaching into `framework` to assemble units by hand is not the normal path:
it means naming an arch the engine is there to decide, and holding a unit before a build
that would hand it back on `BuildResult.Unit` anyway. Do it only when a caller genuinely
needs something no entrypoint exposes, and treat that need as a missing engine verb
([engine.md](engine.md) §No dagger verbs outside `engine/`).

`framework.Units(cfg, modnames, arch)` turns config into `[]*BuildUnit` — one unit per
selected module, all built for the given arch. The arch arrives already resolved by the
engine entrypoint that is about to build ([engine.md](engine.md) §Arch targets); this
layer writes it into each unit and never asks what it is for. It returns the slice
directly: there is no attempt wrapper in the build path,
because `BuildAttempt` is now an **output** type owned by `srv` (see
[platform-server.md](platform-server.md)), not an input the engine consumes.

Each `BuildUnit` is resolved by `unitFromModule`, which calls `FindFramework` on the
module's `framework` name and stores the resolved `Framework` in `BuildUnit.Framework`
(architecture's "unit carries the resolved framework, not a name"). The engine reads
`unit.Framework`, asks it for a `Plan`, and drives the steps; the
[engine spec](engine.md) owns that execution.
