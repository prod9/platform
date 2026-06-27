# Architecture — the build pipeline and how to organize it

Status: **design-of-record.** Drives package layout and the object model. New code
conforms; existing code migrates toward it.

## The pipeline

`platform` is a thin pipeline. Each stage has one job and hands a fully-formed value
to the next — there is no shared mutable state, no stage reaching back into a prior
one's internals.

```
platform.toml ─parse─▶ config model ─interpret─▶ jobs model ─▶ engine ─▶ build
```

| Stage         | Package      | Responsibility                                             |
| ------------- | ------------ | ---------------------------------------------------------- |
| parse         | `project/`   | read `platform.toml` → the **config model**                |
| config model  | `project/`   | `Project` / `Module` — parsed, defaulted, inferred config  |
| ops model     | `ops/`       | the `[ops]` delivery target (`Image`/`Tag`/`Vars`)         |
| interpret     | `builder/`   | config → **jobs model** (one `Job` per selected module)    |
| jobs model    | `builder/`   | `Job` — a complete, **data-only** build definition         |
| engine        | `engine/`    | the Dagger session/pool the jobs run on                    |
| build         | `builder/`   | concrete builders + execution: `[]Job` + engine → images   |

## How to think about it (the durable principle)

**Construct a complete definition tree once, after parsing — then execute it.** By the
time work reaches the engine, every fact the build needs already lives in the `Job`
struct: workdir, image name, build dir, env, command, and the **arch target**. The
build step reads fields; it does not get told things through call arguments.

This yields two standing rules:

- **Model the object, don't thread the argument.** If you find yourself adding a
  parameter to carry a fact through several calls, that fact belongs in the struct the
  pipeline already passes. A method growing a long argument list is the smell — stop and
  put the data where it lives. (`Job` already carries `BuildDir`/`ImageName`; the arch
  target sits right beside them.) The command declares *intent* once (a `Purpose`:
  local vs publish); the model resolves intent into concrete `Job` fields.

- **`Job` is data, not behavior.** It holds a builder *name* (a string), never a
  `Builder` interface. That decoupling is what lets `jobs` / `engine` / `builder` stay
  separate packages without an import cycle: the build stage resolves the named builder
  at execution time.

## No grab-bag packages

Every package is named for a responsibility. `core/`, `util/`, `common/`, `helpers/`,
`misc/` are banned (general-coding) — they absorb anything vaguely shareable and rot.
Domain packages live at the top level by their own name: `baseline/`, `gitops/`,
`dsl/`, `ops/` — never nested under a `core/` catch-all.

## Arch target (local vs publish)

Two kinds of build run from one machine: fast local iteration and server-bound
publish/deploy. They need different architectures, so the config splits them:

- `local_arch` — default `auto` (tracks the host arch; fast native local builds).
- `publish_arch` — default `amd64` (matches the servers, so an arm laptop never ships
  an unrunnable image).

Values are bare archs (`auto` | `amd64` | `arm64`) — the OS is always `linux` for these
containers, so `Job.Platform` is derived as `"linux/" + arch` (or the host arch for
`auto`). The deprecated single-target `platform` key stays readable for backward
compatibility and seeds `local_arch` when unset.

`build` / `preview` / `export` / `ls` build with `local_arch`; `publish` / `deploy`
build with `publish_arch`. The **infra-package manifest artifact** (`ops publish`) has
no executable and no arch — it is untouched by this.
