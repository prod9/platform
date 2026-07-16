# Architecture — the build pipeline and how to organize it

Status: **design-of-record.** Drives package layout and the object model. New code
conforms; existing code migrates toward it.

## The pipeline

`platform` is a thin pipeline. Each stage has one job and hands a fully-formed value
to the next — there is no shared mutable state, no stage reaching back into a prior
one's internals.

```
platform.toml ─parse─▶ config model ─interpret─▶ BuildAttempt ─▶ engine.Build ─▶ images
```

| Stage        | Package      | Responsibility                                                    |
| ------------ | ------------ | ----------------------------------------------------------------- |
| parse        | `project/`   | read `platform.toml` → the **config model**                       |
| config model | `project/`   | `Project` / `Module` — parsed, defaulted, inferred config |
| interpret    | `framework/` | config → a **`BuildAttempt`** (one `BuildUnit` per module)        |
| build model  | `framework/` | `BuildAttempt` has-many `BuildUnit` — the resolved build def      |
| strategies   | `framework/` | the `Framework` implementations — per-stack build knowledge       |
| engine       | `engine/`    | the Dagger `Engine` + executor — runs the attempt's units         |

## How to think about it (the durable principle)

**Construct a complete definition tree once, after parsing — then execute it.** By the
time work reaches the engine, every fact the build needs already lives in the
`BuildUnit`: workdir, image name, build dir, env, command, the resolved framework, and
the **arch target**. The engine reads fields; it does not get told things through call
arguments.

This yields three standing rules:

- **Model the object, don't thread the argument.** If you find yourself adding a
  parameter to carry a fact through several calls, that fact belongs in the struct the
  pipeline already passes. A method growing a long argument list is the smell — stop and
  put the data where it lives. (`BuildUnit` already carries `BuildDir`/`ImageName`; the
  arch target sits right beside them.) The command declares *intent* once (a `Purpose`:
  local vs publish); interpret resolves intent into concrete `BuildUnit` fields.

- **The unit carries the resolved framework, not a name.** `BuildUnit.Framework` holds the
  `Framework` value `FindFramework` resolved at interpret time. A `BuildUnit` is an in-memory
  arg-bag the engine executes — not a record it serializes — so collapsing the framework to
  a string buys nothing: the engine would re-resolve it either way, and (see below) the
  import graph doesn't need it. **Persistence is a downstream concern, not the model's.**
  If a build is ever written to a database, that db package owns a shim/record type that
  maps *from* `BuildUnit`; `framework`/`engine` never contort to be row-shaped.

- **The consumer defines what it needs of the engine.** A framework needs only a Dagger
  client to build, so `Build` takes the raw `*dagger.Client` as a parameter — `framework`
  never imports `engine` at all. **That** is what keeps the import graph one-directional
  (not any stringly-typing): `engine → framework → project`.

## Two models, both data

`Project`/`Module` (config) and `BuildAttempt`/`BuildUnit` (resolved) are *both* data —
the input config and the lower, interpreted model derived from it (source vs IR). The
*behavior* lives elsewhere: the `Framework` strategies (per-stack knowledge) and `engine`
(the runtime that runs them). Keep the two data models distinct — a `BuildUnit` does not
reach back into the `Project` it came from; it carries denormalized copies of what it
needs (`Excludes`, `Repository`), so the build stage stays self-contained.

## No grab-bag packages

Every package is named for a responsibility. `core/`, `util/`, `common/`, `helpers/`,
`misc/` are banned (general-coding) — they absorb anything vaguely shareable and rot.
Domain packages live at the top level by their own name: `framework/`, `gitops/`,
`dsl/`, `engine/` — never nested under a `core/` catch-all.

## Package layout (target)

A **`Framework` is the sole owner of a project type** — it recognizes itself, scaffolds
itself, and builds itself. Only two things sit outside a framework: the `platform.toml`
data model, and the `init` command's human orchestration. The packages form an acyclic
graph `project ← framework/scaffold ← framework ← cmd`:

- `project/` — the `platform.toml` model, both directions: `Generate` and the surgical
  `[vars]` merge. The publish target is not a stored section: a module's image is inferred
  per-module (`InferImageBase`) and the tag derives from the release strategy; only the
  top-level `[vars]` table is carried, fed to `render`.
- `framework/scaffold/` — **the one** files/templating mechanism: render templates with
  data, write files. Generic — no discover, no orchestration, no per-type data or "spec".
- `framework/` — the `Framework` interface (`Discover`, `Scaffold`, `Build`), the concrete
  frameworks, the package-level `Discover(wd)` resolver, the interpret stage (config →
  `BuildUnit`s), and the per-stack build strategies. **Stack discovery is a scaffold-time
  concern** — the build path reads `[modules]`, never re-discovers. The `Infra` framework
  embeds its own baseline assets, version pins, and destination routing here, and its
  `Build` renders `apps/` (CUE + `.platform`) into a `FROM scratch` image (see
  [infra-publishes-as-plain-image-retire-oras](../decisions/2026-07-05-infra-publishes-as-plain-image-retire-oras.md)).
- `cmd/init` — the human orchestration of `platform init`: gather operator inputs →
  `framework.Discover` → `fw.Scaffold` → confirm → write. No app-vs-infra branch; the
  distinction is pure `Scaffold` polymorphism (`Infra.Scaffold` simply contributes more).
- `engine/` — the Dagger runtime: discovers the available Dagger **runners** and distributes
  each attempt's units across them.
- `gitops/` — infra **render** only (CUE/`.platform` → manifest `Tree`). Publishing is the
  ordinary `publish` path now that infra is a framework; the oras packer is retired.
- `dsl/`, `releases/`, `git/` (formerly `gitctx/`+`gitcmd/`), `internal/` — unchanged in role.

The former `baseline/` and top-level `scaffold/` packages are **absorbed**, not surviving
packages: `baseline/`'s templating folds into `framework/scaffold/` and its embedded infra
files + version pins + routing move into the `Infra` framework; `scaffold/`'s mechanism
folds into `framework/scaffold/`, its discovery into `framework/`, and its orchestration
into `cmd/init`.

Command surface: `init  build  configure  exec  export  ls  preview  publish  release
render  clean  vanity`. `clean` prunes the local Dagger build cache (first-line cache
diagnostics — see [`../guides/troubleshooting-build-cache.md`](../guides/troubleshooting-build-cache.md)). `publish` is uniform (infra is just a framework module); `render` emits the
`k8s/` tree for the serverless `kubectl apply` path. No `ops` group; no `discover` or
`bootstrap` — re-run `init` to see detected modules.

## Arch target (local vs publish)

Two kinds of build run from one machine: fast local iteration and server-bound
publishing. They need different architectures, so the config splits them:

- `local_arch` — default `auto` (tracks the host arch; fast native local builds).
- `publish_arch` — default `amd64` (matches the servers, so an arm laptop never ships
  an unrunnable image).

Values are bare archs (`auto` | `amd64` | `arm64`) — the OS is always `linux` for these
containers, so `BuildUnit.Arch` is derived as `"linux/" + arch` (or the host arch for
`auto`). The deprecated single-target `platform` key stays readable for backward
compatibility and seeds `local_arch` when unset.

`build` / `preview` / `export` / `ls` build with `local_arch`; `publish`
builds with `publish_arch`. The infra manifest artifact is a `FROM scratch` image (YAML
only, no executable) — arch is irrelevant to it, so it is untouched by this.

## Infra delivery is a framework, not a separate pipeline

Rendering and shipping the infra repo is **the same build pipeline**, not a parallel one.
Infra is a framework: its `Build` renders the `apps/` CUE + `.platform` directives (via
the linked CUE evaluator + `dsl`) into a manifest tree and packs that tree into a `FROM scratch`
image. So **infra publish is the ordinary `publish` verb** — Dagger builds the image, Dagger
pushes it with the same local-docker credentials as any app image. There is no bespoke OCI
pusher and no separate `ops publish`.

Flux consumes the plain image via `OCIRepository` + a `layerSelector` that extracts the
`application/vnd.oci.image.layer.v1.tar+gzip` layer; kustomize-controller applies the docs.
Compatibility lives on the **consumer** side (a stock, in-production Flux path), which is why
Dagger's native image output suffices and `oras-go` + the Flux-media-type packer are retired.
Full rationale:
[infra-publishes-as-plain-image-retire-oras](../decisions/2026-07-05-infra-publishes-as-plain-image-retire-oras.md).
The `gitops` package keeps only the **render** half (manifest `Tree`); the serverless
`render` → `kubectl apply` path is unaffected.
