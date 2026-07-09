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

| Stage        | Package    | Responsibility                                                    |
| ------------ | ---------- | ----------------------------------------------------------------- |
| parse        | `project/` | read `platform.toml` → the **config model**                       |
| config model | `project/` | `Project` / `Module` / `Ops` — parsed, defaulted, inferred config |
| interpret    | `builder/` | config → a **`BuildAttempt`** (one `BuildUnit` per module)        |
| build model  | `builder/` | `BuildAttempt` has-many `BuildUnit` — the resolved build def      |
| strategies   | `builder/` | the `Builder` implementations — per-stack build knowledge         |
| engine       | `engine/`  | the Dagger `Engine` + executor — runs the attempt's units         |

## How to think about it (the durable principle)

**Construct a complete definition tree once, after parsing — then execute it.** By the
time work reaches the engine, every fact the build needs already lives in the
`BuildUnit`: workdir, image name, build dir, env, command, the resolved builder, and
the **arch target**. The engine reads fields; it does not get told things through call
arguments.

This yields three standing rules:

- **Model the object, don't thread the argument.** If you find yourself adding a
  parameter to carry a fact through several calls, that fact belongs in the struct the
  pipeline already passes. A method growing a long argument list is the smell — stop and
  put the data where it lives. (`BuildUnit` already carries `BuildDir`/`ImageName`; the
  arch target sits right beside them.) The command declares *intent* once (a `Purpose`:
  local vs publish); interpret resolves intent into concrete `BuildUnit` fields.

- **The unit carries the resolved builder, not a name.** `BuildUnit.Builder` holds the
  `Builder` value `FindBuilder` resolved at interpret time. A `BuildUnit` is an in-memory
  arg-bag the engine executes — not a record it serializes — so collapsing the builder to
  a string buys nothing: the engine would re-resolve it either way, and (see below) the
  import graph doesn't need it. **Persistence is a downstream concern, not the model's.**
  If a build is ever written to a database, that db package owns a shim/record type that
  maps *from* `BuildUnit`; `builder`/`engine` never contort to be row-shaped.

- **The consumer defines the engine interface.** A builder needs only `Client()` /
  `Context()` off the engine, so `builder` declares a small `Session` interface and the
  strategies consume *that*. `engine`'s concrete `Engine` satisfies it implicitly —
  `builder` never imports `engine` to get it. **That** is the cycle-breaker (not any
  stringly-typing): the graph is `engine → builder → project`, one direction.

## Two models, both data

`Project`/`Module` (config) and `BuildAttempt`/`BuildUnit` (resolved) are *both* data —
the input config and the lower, interpreted model derived from it (source vs IR). The
*behavior* lives elsewhere: the `Builder` strategies (per-stack knowledge) and `engine`
(the runtime that runs them). Keep the two data models distinct — a `BuildUnit` does not
reach back into the `Project` it came from; it carries denormalized copies of what it
needs (`Excludes`, `Repository`), so the build stage stays self-contained.

## No grab-bag packages

Every package is named for a responsibility. `core/`, `util/`, `common/`, `helpers/`,
`misc/` are banned (general-coding) — they absorb anything vaguely shareable and rot.
Domain packages live at the top level by their own name: `baseline/`, `gitops/`,
`dsl/`, `scaffold/`, `engine/` — never nested under a `core/` catch-all.

## Package layout (target)

The streamline collapses the old app-side/infra-side split into one config spine with the
actions hanging off it. New code conforms; existing code migrates toward it.

- `project/` — the config spine. The `Ops` delivery model (`[ops]` image/tag/vars) lives
  here as a sub-model; there is no separate `ops/` package.
- `scaffold/` — `platform init`: one plan builder, app and infra unified (the component
  picker decides which, not a mode flag). Was `bootstrapper/`; consumes `baseline/` (the
  embedded seed files) and `builder` stack-discovery to populate `[modules]`.
- `builder/` — interpret config → `BuildUnit`s; the per-stack `Builder` strategies; and
  **stack discovery** (a scaffold-time concern — the build path reads `[modules]`, never
  re-discovers). Includes the `infra` builder: renders `apps/` (CUE + `.platform`) into a
  `FROM scratch` image (see
  [infra-publishes-as-plain-image-retire-oras](../decisions/2026-07-05-infra-publishes-as-plain-image-retire-oras.md)).
- `engine/` — the Dagger runtime: discovers the available Dagger **runners** and distributes
  each attempt's units across them.
- `gitops/` — infra **render** only (CUE/`.platform` → manifest `Tree`). Publishing is the
  ordinary `publish` path now that infra is a builder; the oras packer is retired.
- `dsl/`, `baseline/`, `releases/`, `gitctx/`, `internal/` — unchanged in role.

Command surface: `init  build  configure  exec  export  ls  preview  publish  release
render  clean  vanity`. `clean` prunes the local Dagger build cache (first-line cache
diagnostics — see [`../guides/troubleshooting-build-cache.md`](../guides/troubleshooting-build-cache.md)). `publish` is uniform (infra is just a builder module); `render` emits the
`k8s/` tree for the serverless `kubectl apply` path. No `ops` group; no `discover` or
`bootstrap` — re-run `init` to see detected modules.

## Arch target (local vs publish)

Two kinds of build run from one machine: fast local iteration and server-bound
publishing. They need different architectures, so the config splits them:

- `local_arch` — default `auto` (tracks the host arch; fast native local builds).
- `publish_arch` — default `amd64` (matches the servers, so an arm laptop never ships
  an unrunnable image).

Values are bare archs (`auto` | `amd64` | `arm64`) — the OS is always `linux` for these
containers, so `BuildUnit.Platform` is derived as `"linux/" + arch` (or the host arch for
`auto`). The deprecated single-target `platform` key stays readable for backward
compatibility and seeds `local_arch` when unset.

`build` / `preview` / `export` / `ls` build with `local_arch`; `publish`
builds with `publish_arch`. The infra manifest artifact is a `FROM scratch` image (YAML
only, no executable) — arch is irrelevant to it, so it is untouched by this.

## Infra delivery is a builder, not a separate pipeline

Rendering and shipping the infra repo is **the same build pipeline**, not a parallel one.
Infra is a builder class: its `Build` renders the `apps/` CUE + `.platform` directives (via
the linked CUE engine + `dsl`) into a manifest tree and packs that tree into a `FROM scratch`
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
