# Architecture — the build pipeline and how to organize it

Status: **design-of-record.** Drives package layout and the object model. New code
conforms; existing code migrates toward it. `platform` is a command-line tool with no
server ([server-removed decision]), and infra repos render Lua rather than CUE
([cue-dropped decision]).

[server-removed decision]: ../decisions/2026-11-25-server-removed-platform-is-a-cli.md
[cue-dropped decision]: ../decisions/2026-11-25-cue-dropped-for-lua-render.md

## The pipeline

`platform` is a thin pipeline driven by `engine`. The CLI's commands call its build verbs;
engine owns their mechanics. There is no shared mutable state.

```
engine.Local ─▶ engine.Build / BuildAndPublish
  local config file ─▶ conf.Model ─▶ BuildUnit ─▶ framework steps ─▶ image
```

| Stage        | Package      | Responsibility                                              |
|--------------|--------------|-------------------------------------------------------------|
| parse        | `conf/`      | read `platform.toml` → the **config model**                 |
| config model | `conf/`      | `Model` / `Module` — parsed, defaulted, inferred config     |
| interpret    | `framework/` | config → **`[]*BuildUnit`** (one per selected module)       |
| build model  | `framework/` | `BuildUnit` — the resolved, self-contained build def        |
| strategies   | `framework/` | the `Framework` implementations — per-stack build knowledge |
| engine       | `engine/`    | build driver                                                |

This is a shared capability pipeline, not an invocation model. The CLI is its one driver,
and which builds publish is the operator's choice, not the pipeline's.
[`execution-modes.md`](execution-modes.md) is canonical for that boundary; this
repository's own release procedure does not constrain other repositories.

## How to think about it (the durable principle)

**Construct a complete definition tree once, after parsing — then execute it.** By the
time work reaches engine's execution core, every fact the build needs already lives in
the `BuildUnit`: workdir, image name, build dir, env, command, the resolved framework, and
the **arch target**. The execution core reads fields; it does not get told things through
call arguments. Engine loads the workdir's configuration through `conf` and interprets it
before entering the step-execution core. [engine.md](engine.md) owns the input and
paired-lifecycle contract.

This yields three standing rules:

- **Model the object, don't thread the argument.** If you find yourself adding a
  parameter to carry a fact through several calls, that fact belongs in the struct the
  pipeline already passes. A method growing a long argument list is the smell — stop and
  put the data where it lives. (`BuildUnit` already carries `BuildDir`/`ImageName`; the
  arch sits right beside them.) Commands declare nothing: they pass an input + module names
  to an engine entrypoint, which loads config, resolves the arch, and interprets concrete
  `BuildUnit` fields.

- **The unit carries the resolved framework, not a name.** `BuildUnit.Framework` holds the
  `Framework` value `FindFramework` resolved at interpret time. A `BuildUnit` is an in-memory
  arg-bag the engine executes — not a record it serializes — so collapsing the framework to
  a string buys nothing: the engine would re-resolve it either way, and (see below) the
  import graph doesn't need it. **Persistence is a downstream concern, not the model's.**
  If a build is ever written to a database, that db package owns a shim/record type that
  maps *from* `BuildUnit`; `framework`/`engine` never contort to be row-shaped.

- **The consumer defines what it needs of the engine.** A framework needs only a Dagger
  client to execute a step, so `Execute` takes the raw `*dagger.Client` as a parameter.
  `framework` never imports `engine` at all. **That** keeps the import graph one-directional
  (not any stringly-typing): `engine → framework → project`.

## Two models, both data

`Model`/`Module` (config) and `BuildUnit` (resolved) are *both* data — the input config
and the lower, interpreted model derived from it (source vs IR). The
*behavior* lives elsewhere: the `Framework` strategies (per-stack knowledge) and `engine`
(the runtime that runs them). Keep the two data models distinct — a `BuildUnit` does not
reach back into the `Model` it came from; it carries denormalized copies of what it
needs (`Excludes`, `Repository`), so the build stage stays self-contained.

## No grab-bag packages

Every package is named for a responsibility. `core/`, `util/`, `common/`, `helpers/`,
`misc/` are banned (general-coding) — they absorb anything vaguely shareable and rot.
Domain packages live at the top level by their own name: `framework/`, `engine/`,
`releases/` — never nested under a `core/` catch-all.

## Package layout (target)

A **`Framework` is the sole owner of a project type** — it recognizes itself, scaffolds
itself, and builds itself. Only two things sit outside a framework: the `platform.toml`
data model, and the `init` command's human orchestration. The packages form an acyclic
graph `conf ← framework/scaffold ← framework ← scaffolding ← cmd`:

- `conf/` — the `platform.toml` model, both directions: `Generate` and the surgical
  `[vars]` merge. The publish target is not a stored section: a module's image is inferred
  per-module (`InferImageBase`) and the tag derives from the release strategy. Two
  top-level tables feed `render`: `[vars]`, and `[rocks]`, which pins the version of each
  Lua rock the infra repo renders with ([lua-render.md](lua-render.md)).

  **`repository` is stored scheme-less** — `github.com/prod9/platform`, the form the ghcr
  mirror and the image-source label are both derived from. Every other form is derived, and
  `conf` owns the derivation: `InferImageBase` for the registry path, `RepositoryURL` for
  the `https://` form a browser needs. A consumer that pastes `Repository` into a URL
  itself emits a broken link — the derivations exist so nobody has to know which form is
  stored.
- `framework/scaffold/` — **the one** files/templating mechanism: render templates with
  data. Generic — no discover, no orchestration, no per-type data; the driver writes
  finished bytes.
- `framework/` — the `Framework` contract (seven methods — [`frameworks.md`](frameworks.md)
  is canonical), the concrete frameworks, the package-level `Discover(wd)` resolver, the
  interpret stage (config → `BuildUnit`s), and the per-stack build strategies. **Stack
  discovery is a scaffold-time concern** — the build path reads `[modules]`, never
  re-discovers. The `Infra` framework owns its destination routing and the universal
  launcher (bytes in the `framework/skel` collection); the cluster baseline it scaffolds
  comes from the templates shipped in the `infra-lua` rock
  ([scaffolding.md](scaffolding.md)). Its render step turns `apps/*.lua` into a
  `FROM scratch` image (see
  [infra-publishes-as-plain-image-retire-oras](../decisions/2026-07-05-infra-publishes-as-plain-image-retire-oras.md)).
- `framework/gitops/` — the Infra framework's **render** machinery (`apps/*.lua` →
  manifest `Tree`), consumed by the Infra framework's render step and the
  `render` command. It is nested under `framework/` like `gowork`, its stack-specific
  companion. Publishing is the ordinary `publish` path now that infra is a framework; the
  oras packer is retired. [lua-render.md](lua-render.md) specifies the sandbox, the rock
  fetch and cache, and `platform.vars`.
- `rocks/` — fetches a pinned Lua rock from ghcr and keeps it unpacked in the XDG cache
  ([lua-render.md](lua-render.md), "Rocks"). It knows nothing of render or
  `platform.toml`: its caller passes the rock's name and version and the registry
  credentials. It imports no other platform package, so `framework/gitops` can consume it
  without a cycle.
- `cmd/` — `main.go` defers to `cmd.Execute()`. `cmd` holds the root Cobra command
  (persistent `-q`/`-v`, and `-f` for an alt `platform.toml`) plus one file per command.
  The `init` command's adapter lives at `cmd/init_cmd.go`; it presents prompts and the
  plan, confirms, and calls `Apply` or `ForceApply`. Command-domain logic does not live here.
  **A file here is a command; anything package-generic goes in `cmd/cmd.go`** — the Go
  convention of naming the package-level file after the package.
- `scaffolding/` — the platform-initialization domain: `Discover` validates the target and
  resolves its framework once; the resulting `Target` exposes its `ScaffoldVars` and builds
  a read-only `Plan`; `Apply` preserves existing files and `ForceApply` overwrites them. Its
  Cobra adapter remains in `cmd/init_cmd.go`; the package owns no command registration.
- `engine/` — the reusable build driver behind the CLI's commands. `Session` + `Run` own
  Dagger execution; the roster owns endpoint discovery and selection. The package knows no
  delivery policy. Multi-unit fan-out stays behind engine build verbs. See
  [`engine.md`](engine.md).
- `releases/`, `git/` (formerly `gitctx/`+`gitcmd/`), `internal/` — unchanged in role.
  `internal/termlog` is platform's terminal narration sink (formerly `buildlog`; see
  [engine.md](engine.md)).

The former `baseline/` and top-level `scaffold/` packages are **absorbed**, not surviving
packages: `baseline/`'s templating folds into `framework/scaffold/` and its embedded infra
files + version pins + routing move into the `Infra` framework; `scaffold/`'s mechanism
folds into `framework/scaffold/`, its discovery into `framework/`, and its orchestration
into `scaffolding/`, behind the `cmd/init_cmd.go` adapter.

Command surface: `init  build  configure  exec  export  ls  preview  publish  release
render  clean  versions  print-config`.
`clean` prunes the local Dagger build cache
(first-line cache diagnostics — see
[`../guides/troubleshooting-build-cache.md`](../guides/troubleshooting-build-cache.md));
`versions` lists the release history. The `publish` command is uniform (infra is just a
framework module); `render` emits the `k8s/` tree for the `kubectl apply` path. No `ops`
group; no `discover` or `bootstrap` — re-run `init` to see detected modules.

**A multi-module verb finishes every module, then fails once.** `build`, `export` and
`publish` fan out. Engine returns the available unit results and joins failures only after
all selected modules finish; a preparation failure cannot fabricate a container-bearing
result. Even when engine returns an error, the command processes the successful results,
does the work each successful unit earned, and returns the failures joined. Bailing on the
first failure would throw away units that had already built and would report a count that
depends on which goroutine finished first. The exit code says whether *anything* failed;
the log says which.

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

`publish` builds with `publish_arch`; other session build verbs use `local_arch`, or
`publish_arch` when `CI` is true, as specified in [engine.md](engine.md). `ls` uses
`local_arch` for its debugging view. The infra manifest artifact is a `FROM scratch` image
(YAML only, no executable) — arch is irrelevant to it, so it is untouched by this.

## Infra delivery is a framework, not a separate pipeline

Rendering and shipping the infra repo uses **the same build pipeline**, not a parallel
one. Infra is a framework: its render step runs the repo's `apps/*.lua` on the embedded,
sandboxed Lua VM against the pinned `infra-lua` rock and packs the resulting manifest
tree into a `FROM scratch` image. Documents are not checked against Kubernetes schemas;
the cluster rejects a bad one when Flux applies it ([validation decision]).
`./platform publish` pushes the image through the same
publish capability as any other module. There is no bespoke OCI pusher and no separate
`ops publish`. [lua-render.md](lua-render.md) is canonical for render.

[validation decision]: ../decisions/2026-11-25-validation-left-to-flux.md

The output contract is component, then file name, then a list of documents: each
`<component>/<filename>` entry of the `Tree` is one file holding one or more YAML
documents, and the infra image carries the whole tree in exactly one layer.

Flux consumes the plain image via `OCIRepository` + a `layerSelector` that extracts the
`application/vnd.oci.image.layer.v1.tar+gzip` layer; kustomize-controller applies the docs.
Compatibility lives on the **consumer** side (a stock, in-production Flux path), which is why
Dagger's native image output suffices and `oras-go` + the Flux-media-type packer are retired.
Full rationale:
[infra-publishes-as-plain-image-retire-oras](../decisions/2026-07-05-infra-publishes-as-plain-image-retire-oras.md).
The `gitops` package keeps only the **render** half (manifest `Tree`); the `render` →
`kubectl apply` path is unaffected.

Delivery is **pull-based**. Platform feeds Flux through the registry and never reconciles
in-cluster state itself, and no platform credential reaches into the cluster: the cluster
pulls everything.
