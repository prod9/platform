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

| Stage        | Package    | Responsibility                                                |
| ------------ | ---------- | ------------------------------------------------------------- |
| parse        | `project/` | read `platform.toml` → the **config model**                   |
| config model | `project/` | `Project` / `Module` — parsed, defaulted, inferred config     |
| ops model    | `ops/`     | the `[ops]` delivery target (`Image`/`Tag`/`Vars`)            |
| interpret    | `builder/` | config → a **`BuildAttempt`** (one `BuildUnit` per module)    |
| build model  | `builder/` | `BuildAttempt` has-many `BuildUnit` — the resolved build def  |
| strategies   | `builder/` | the `Builder` implementations — per-stack build knowledge     |
| engine       | `engine/`  | the Dagger `Pool` + the executor — runs the attempt's units   |

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
  strategies consume *that*. `engine`'s concrete `Pool` satisfies it implicitly —
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
`dsl/`, `ops/`, `engine/` — never nested under a `core/` catch-all.

## Arch target (local vs publish)

Two kinds of build run from one machine: fast local iteration and server-bound
publish/deploy. They need different architectures, so the config splits them:

- `local_arch` — default `auto` (tracks the host arch; fast native local builds).
- `publish_arch` — default `amd64` (matches the servers, so an arm laptop never ships
  an unrunnable image).

Values are bare archs (`auto` | `amd64` | `arm64`) — the OS is always `linux` for these
containers, so `BuildUnit.Platform` is derived as `"linux/" + arch` (or the host arch for
`auto`). The deprecated single-target `platform` key stays readable for backward
compatibility and seeds `local_arch` when unset.

`build` / `preview` / `export` / `ls` build with `local_arch`; `publish` / `deploy`
build with `publish_arch`. The **infra-package manifest artifact** (`ops publish`) has
no executable and no arch — it is untouched by this.
