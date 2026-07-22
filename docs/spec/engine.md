# Engine

Status: **design-of-record.** The `engine/` package — the Dagger execution layer that runs
a unit's planned steps and pushes the resulting images. Sits at the tail of the pipeline
([`architecture.md`](architecture.md)): `[]*BuildUnit ─▶ engine.Run ─▶ images`.

## What the Engine is

`Engine` is a process-wide handle to a fleet of Dagger runners — shaped like `sql.DB`: a
concurrency-safe connection pool, dialed lazily and reused, built once from config and
shared across the process. It orchestrates two single-purpose units:

- **runners** ([`runners.go`](../../engine/runners.go)) — which endpoints exist.
- **clients** ([`clients.go`](../../engine/clients.go)) — one reused, ping-checked
  `*dagger.Client` per endpoint.

`Engine` itself holds no lock — only an atomic round-robin `cursor`. The happy path is
discover → pick → get.

### Lifecycle

| Call                       | Role                                                              |
| -------------------------- | ---------------------------------------------------------------- |
| `New(cfg)`                 | build the handle from an fx config source; dials nothing yet     |
| `NewContext(ctx, eng)`     | carry `eng` on a context so runs resolve it                       |
| `FromContext(ctx)`         | Must-style fetch; panics if absent (engine is a precondition)    |
| `LookupFromContext(ctx)`   | comma-ok fetch                                                    |
| `Client(ctx)`              | next endpoint round-robin → a live client (ad-hoc: `ls`/preview) |
| `Clean(ctx)`               | prune every fleet engine's local cache (drives `platform clean`) |
| `Close()`                  | tear down every dialed connection; call once at shutdown         |

Commands open **one** engine, defer `Close`, and stash it on the context via `NewContext`;
downstream runs pull it back with `FromContext`. Mirrors fx/data's request-scoped
`*sqlx.DB`. One shared `*Engine` serves every concurrent run — it is concurrency-safe via
the atomic round-robin cursor, and `NewRun` does no engine work at all (a client is
grabbed at the first `Next`), so there is never a reason to open a second.

## Runner discovery

`runners` resolves the configured Dagger endpoints via DNS — no k8s API, no RBAC:

| Config               | Default | Meaning                                                     |
| -------------------- | ------- | ----------------------------------------------------------- |
| `DAGGER_ENGINE`      | unset   | headless-Service DNS of the engine pool                     |
| `DAGGER_ENGINE_PORT` | `1234`  | engine pod port (mirrors `apps/dagger-engine.cue`)          |

`Hosts(ctx)` looks up the DNS name and returns one `tcp://<addr>:<port>` per resolved pod,
sorted for stable round-robin. It reports **only what it finds**:

- `DAGGER_ENGINE` unset → empty slice, no lookup.
- DNS resolves to nothing → empty slice.
- lookup failure → a real error, surfaced.

Falling back to a local engine is **not** the runner's decision — it reports emptiness and
the core decides. `Engine.resolveHosts` maps an empty result to a single empty-string
host, and `dialEngine` reads an empty host as "let Dagger auto-provision and reuse the
local engine." So unset `DAGGER_ENGINE` is an explicit operator choice for local, never
inferred.

The resolver caches per the DNS record TTL, so a new pod becomes selectable as soon as DNS
reflects it — no restart.

### Client pool

`clients` caches one `*dagger.Client` per host. `Get` validates a cached client with a
cheap `Version()` ping and redials when the engine has gone (graceful DNS removal or a
crash), so callers always receive a live client — no separate prune step, nothing closed
mid-build. The lock is held only around map reads/writes, never across a dial or ping;
concurrent dial races keep one winner and close the loser. Liveness during a run is the
ping's job; `Close` only runs at shutdown.

## `Run` — one unit, one step at a time

`engine.NewRun(eng, unit)` is a **single-unit** run. It asks the unit's framework for a
`Plan`, then exposes a cursor:

```go
run := engine.NewRun(eng, unit)
for run.Next(ctx) { }          // drives exactly one Step per call
```

Each `Next` grabs a client (`Engine.Client(ctx)` — round-robin, so concurrent runs spread
across the fleet), calls `Framework.Execute` for the current `Step` with the previous
step's container, forces the work eagerly with `.Sync()`, and **times** the step across
that boundary. `Next` returns false at the end of the plan or on failure; the error is
reported on the event stream, not by aborting siblings. The whole run is wrapped in
`context.WithTimeout(ctx, unit.Timeout)`.

Clone (repo-prep) and Publish are **engine brackets** around this loop, not framework
steps — cloning is not any stack's build knowledge, and pushing is the engine's registry
concern.

### One event stream

A run reports everything on **one** channel. There is no second channel to synchronize
against — no separate `Logs()`, no `Updates()`, no snapshot-plus-delta:

```go
type Event struct {
    At   time.Time
    Unit *BuildUnit
    Step Step
    Kind EventKind   // StepStarted | StepDone | Log | Failed
    Line string      // set on Kind == Log
    Err  error       // set on Kind == Failed
}

func (r *Run) Events() <-chan Event
```

Everything else is a **fold** of this stream: a step's elapsed time is
`StepDone.At − StepStarted.At`; a run's current state is the reduction of its events so
far. `StepResult`, `Update`, and `Result` types are collapsed into `Event` + fold, and
`Snapshot`/`Done` are dropped outright — execution moves to a worker that writes events to
the database and the webui reads them back, so there is no late-joining live observer to
catch up.

**Log capture rides `Container.Stdout`/`Stderr`, never `WithLogOutput`.** Per-unit
retrieval is incremental and per-step: each step's output is read as that step finishes,
with no re-execution (Dagger caches the walk), landing exactly on the `.Sync()` boundary
`Next` already has — so `Kind == Log` events flush per step on the one stream. It demuxes
cleanly across units sharing a pooled client, so the client pool is untouched by log
capture. `WithLogOutput` is the Dagger CLI *subprocess's* stderr pipe — rendered TUI text,
never demuxable — and is not a capture path.

**`buildlog` is not build-progress.** `internal/buildlog` is platform's own narration of
what *platform* is doing; the `Event` stream is what the *build* is doing. They coincide
only on a machine-local CLI run, and must not be merged.

### The built container is hidden

The `*dagger.Container` a run produces is **engine-internal**. It is bound to the client
that built it, and it is carried so that steps chain and `Publish` can push it — but
consumers read the event stream, not the container. The lone exception is `preview`, whose
post-build tunnel genuinely needs the container handle; it gets it by an explicit method.
That is the one non-event thing a run exposes.

## The execution boundary

Three properties define an engine, and together they fix where it ends:

- **It executes, it does not decide.** Given a unit, it produces an artifact or a failure.
- **It is capacity.** There are N of them and work is dispatched across them.
- **It is domain-blind.** It knows nothing of repos, tags, queues, or that this is build
  #47 triggered by a push. The engine boundary is exactly where domain knowledge stops.

Everything else a CI/CD server does — deciding what should run, finding free capacity,
recording what happened — is coordination *around* engines.

### Two scheduling decisions, two layers

Scheduling splits in two and the halves must not meet:

| Decision                 | Owner  | Inputs                                   |
| ------------------------ | ------ | ---------------------------------------- |
| which build runs next    | worker | pending records, concurrency policy      |
| which runner executes it | engine | `runners.Hosts()`, client health, cursor |

**The worker never sees a host address.** If one leaks upward the boundary is gone and
two schedulers begin fighting over the same capacity.

### No dagger verbs outside `engine/`

Callers must never *express container operations*: no `WithExec`, no `WithDirectory`, no
`Sync`. Each of those is an execution decision, and authoring one outside the engine moves
part of the build definition out of the layer that owns it. The tell is a dagger
constructor appearing anywhere outside `engine/`.

This is a rule about **logic, not types**. Go type inference means `c := engine.Build(…)`
compiles with no dagger import at all, so "does this package import dagger" is the wrong
test — a caller can hold what a run returns opaquely and still respect the boundary.

The requirement this places is on the engine's API surface, not its return types: it must
expose domain verbs complete enough that nobody needs to reach past them. **If a caller
ever has to chain two engine calls with its own container work in between, that gap is a
missing engine verb** — that is the working test for whether the boundary holds.

## Fan-out lives at the cmd layer

Multi-unit fan-out is **not** an engine concern. `engine/multiplex` owns it:
`multiplex.NewRun(eng, units...)` wraps more than one unit, drives a `Run` per unit
against the one shared `*Engine`, and merges their per-unit events into a single `Event`
stream (each `Event` carries its `Unit`, so the merge is unambiguous). The generic
`multiplexer` worker lives here, unexported — it moved out of `engine` proper.

`cmd` instantiates a multiplex run, drives it, and assembles the operator-facing output
from the merged stream. A per-unit failure surfaces as that unit's `Failed` event and
never aborts its siblings.

## Publish

`Publish` pushes a successfully-built container, reusing the client that built it so the
registry secret is minted by the engine that owns the container, and logs the image via
`buildlog.Image`. `publish` composes the ordinary path — build the units at the publish
arch, suffix each `ImageName` with the release tag, run, then push — and the
`[]BuildAttempt` records of what shipped are assembled by `srv`
([platform-server.md](platform-server.md)), not by the engine.

`release` (cut a tag) and `publish` (build + push) are orthogonal — neither implies the
other, and there is no `deploy` verb. See
[delivery-verbs-are-orthogonal](../decisions/2026-07-05-delivery-verbs-are-orthogonal.md)
for the full rationale.

## Registry credentials

`Publish` reads three fx env-config values off the engine's config source:

| Config              | Role                          |
| ------------------- | ----------------------------- |
| `REGISTRY`          | registry host for auth        |
| `REGISTRY_USERNAME` | registry user                 |
| `REGISTRY_PASSWORD` | registry secret (set via `client.SetSecret`, never inlined) |

When `REGISTRY_USERNAME` is empty, `Publish` skips `WithRegistryAuth` entirely — Dagger
then pushes with the **local docker credentials** (osxkeychain). That is the local-publish
path: a `platform publish` on a laptop needs no `REGISTRY_USERNAME`/`PASSWORD`, only a
docker login to ghcr. The env creds are for a server driver with no local docker config.

## Arch targets

**The arch question is not where you are standing, it is whether the image outlives the
box that built it.** A build whose output is pushed to a registry runs somewhere else and
must carry that somewhere's arch; a build-and-discard runs here and takes the host arch
for speed. "Local vs publish" reads as a location and breaks the moment a non-publish
verb runs on a build-server, where nothing is local — so there is no `Target` type and no
declared intent. There is a resolved arch string and the engine entrypoint that resolves
it:

| Entrypoint                    | Arch                                    |
| ----------------------------- | --------------------------------------- |
| `BuildAndPublish`             | `publish_arch` — pushing *is* the answer |
| `Build(ctx, cfg, modnames)`   | `local_arch`, or `publish_arch` when `CI` is set |

Engine entrypoints take `cfg` + module names and construct the units themselves; the arch
rule is engine-internal and unexported, because it is only ever an input to an entrypoint
that is about to build. Callers never name an arch, and `preview`/`exec` read what they
need off `BuildResult.Unit` after the build rather than holding a unit before it.

`framework.Units` receives the resolved arch and writes it into each `BuildUnit`
([`framework/unit.go`](../../framework/unit.go)) — the engine then reads the field, never
a call argument. Bare archs become `linux/<arch>`; `auto` tracks `runtime.GOARCH`;
`local_arch` defaults to `auto` and `publish_arch` to `amd64`. The infra `FROM scratch`
manifest image carries no executable, so arch is irrelevant to it.
