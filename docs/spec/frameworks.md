# Frameworks

Status: **design-of-record.** A `Framework` is the **sole owner of a project type** — it
recognizes itself (`Discover`), scaffolds itself (`Scaffold`), and builds itself (`Build`).
This spec owns the per-stack strategies, stack discovery, and the shared Wolfi base; it
sits at the `interpret`/`strategies` stages of the pipeline. The
[architecture spec](architecture.md) frames the pipeline and the two data models,
[engine](engine.md) owns execution, and [scaffolding](scaffolding.md) owns the scaffold
mechanism and `cmd/init` orchestration — read [architecture.md](architecture.md) first.

## The `Framework` contract

A framework is a stateless value (an empty struct) implementing `framework.Framework`. It
carries per-stack knowledge and nothing else — no config, no engine handle, no build
state. Six methods:

| Method                                            | Returns   | Role                                                      |
| ------------------------------------------------- | --------- | --------------------------------------------------------- |
| `Name() string`                                   | id        | Stable id (`go/basic`, `pnpm/static`, …); `[modules]` key |
| `Layout() Layout`                                 | shape     | `basic` \| `workspace` — module topology                  |
| `Discover(wd string) bool`                        | detect    | True if this stack owns `wd` (scaffold-time only)         |
| `RequiredScaffoldInputs(wd) []string`             | inputs    | Operator inputs to prompt at init, by name (usually nil)  |
| `Scaffold(ctx, wd, repo, dagger, inputs) Spec`    | seed      | The framework's full, **resolved** contribution (below)   |
| `Build(ctx, client, *BuildUnit)`                  | container | Build the module → a synced `*dagger.Container`           |

`Build` reads a fully-resolved `BuildUnit` (workdir, arch, env, command, asset dirs,
image name, vars) and returns a synced container. It is handed the raw `*dagger.Client`,
not the engine — per architecture's consumer-defined-interface rule, the strategy needs
only a client. `Discover` and `Scaffold` are scaffold-time: the build path reads
`[modules]` (which pins `Name`), it never re-discovers.

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
| 3 | `PNPMWorkspace` | `pnpm/workspace` | `workspace` | `interpreted` | `pnpm-workspace.yaml` / `pnpm-workspaces.yaml`|
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
- **Static before basic** (5 before 6) — an Astro project carries `pnpm-lock.yaml` too;
  the `astro.config.mjs` signal is the more specific one and must be checked first, else
  every Astro repo detects as `pnpm/basic`.
- **Dockerfile last** — the escape hatch. It bypasses the Wolfi base and package
  conventions and emits a runtime warning; every language-specific framework is preferred,
  so it only wins when nothing else matched.

## The shared base

Every framework except `Dockerfile` (own `FROM`) and `Infra` (`FROM scratch`) starts from
`BaseImageForUnit` — Chainguard's Wolfi base (`cgr.dev/chainguard/wolfi-base`), small,
glibc-free, regularly patched, shared across all language stacks.

- **Pinned by digest.** `BaseImageName` pins the multi-arch index digest; Dagger picks the
  per-platform manifest at build time. Chainguard `:latest` floats, so reproducibility
  wins. Refresh manually on a monthly cadence to absorb base-layer CVEs; userland is
  refreshed every build via `apk update && apk upgrade`.
- **apk cache mount.** `/var/cache/apk` mounts the persistent `platform-apk-cache` volume
  so package pulls survive across builds.
- **`CacheBuster`.** A const written into the image (`/<CacheBuster>`) to force Dagger and
  Docker to invalidate cached base layers across all environments. Bumped in lockstep with
  `BaseImageName` (its hex is the first 8 chars of the digest) so a base refresh always
  re-pulls; can be bumped alone if Chainguard ships a bad image at the same digest.

The base lays down a fixed FHS-style tree so an operator shelling in always finds things
in the same place: `SrcDir` (`/platform/src`, build workspace), `BinDir` (`/platform/bin`,
on `PATH`), `RunDir` (`/platform/run`, runtime workdir). Package sets are applied via
`withBuildPkgs` (`build-base git curl bash` + extras) for the build stage and
`withRunnerPkgs` (`ca-certificates curl netcat-openbsd tzdata` + extras) for the runner;
`withCaddyServer` adds Caddy for the static family.

## Test-in-build is a hard gate

The Go frameworks run the module's tests **inside the image build**, before the compile
step: `GoBasic` execs `go test -v ./...`, `GoWorkspace` execs `go test -v` across every
workspace module. Because Dagger fails the build on a non-zero exec, **green tests are a
baked-in, non-configurable precondition of a Go image** — a red suite is a failed build,
and there is no skip-tests opt-out. Full rationale:
[test-in-build-is-a-hard-gate](../decisions/2026-07-05-test-in-build-is-a-hard-gate.md).

## Stack notes

- **Go** — pins the exact toolchain from `go.mod`/`go.work` via native `GOTOOLCHAIN`
  (`withGoVersion`); mounts per-version module and build caches (`withGoCaches`). go.mod/
  go.sum (and every member's in workspace) are copied and `go mod download`-ed before the
  full source, so the dependency layer keys on manifests alone. Runner carries only the
  compiled binary.
- **pnpm** — Node comes from nodejs.org via `tj/n` (pinned `NodeVersion`), pnpm via Node's
  corepack (pinned `PNPMVersion`) — never from distro packages (see the project's
  Node/pnpm provisioning rule). `pnpm/basic` and `pnpm/workspace` serve via bare `node`;
  `pnpm/static` serves the built bundle with Caddy `file-server`. Workspace runner marks
  `RunDir` as ESM (`withPNPMModuleFix`).
- **Dockerfile** — `host.DockerBuild` on the user's `Dockerfile`; env becomes build args.
  Discouraged: bypasses Wolfi, the apk cache, and package conventions; warns at build
  time.
- **Infra** — `Build` calls `gitops.Render` in-process (CUE + `.platform` → manifest
  tree), then writes each file into a `client.Container()` with no `From` (scratch). The
  published layer is a tar+gzip of exactly those files, which Flux's `OCIRepository`
  `layerSelector` extracts; kustomize-controller applies the YAML. Infra delivery is the
  ordinary `publish` verb — see
  [infra-publishes-as-plain-image-retire-oras](../decisions/2026-07-05-infra-publishes-as-plain-image-retire-oras.md).

## Frameworks and the build model

`AttemptFrom` (in `attempt.go`) turns config into a `BuildAttempt` — one `BuildUnit` per
selected module, all sharing a `Purpose` (`LocalBuild` | `PublishBuild`) that pins the
target arch. Each `BuildUnit` is resolved by `unitFromModule`, which calls `FindFramework`
on the module's `framework` name and stores the resolved `Framework` in `BuildUnit.Framework`
(architecture's "unit carries the resolved framework, not a name"). The engine reads
`unit.Framework` and calls `Build`; the engine spec owns that execution.
