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

`NewRun(eng, unit, caller)` is a **single-unit** run, and it is **engine-internal** — the
domain verbs below open runs, no caller does. It asks the unit's framework for a `Plan`,
then exposes a cursor:

```go
run := NewRun(eng, unit, caller)   // caller may be nil; the run injects its own acc
for run.Next(ctx) { }              // drives exactly one Step per call
```

The **first** `Next` grabs the run's client (`Engine.Client(ctx)`) and every later step
reuses it: a container is bound to the client that built it, so one run's steps can never
be spread across the fleet. Round-robin spreads whole *runs*, not steps.

Each `Next` calls `Framework.Execute` for the current `Step` with the previous step's
container, forces the work eagerly with `.Sync()`, and **times** the step across that
boundary. `Next` returns false at the end of the plan or on failure; the error is held on
the run and reported to the observer, not by aborting siblings. The whole run is wrapped in
`context.WithTimeout(ctx, unit.Timeout)`.

Clone (repo-prep) and Publish are **engine brackets** around this loop, not framework
steps — cloning is not any stack's build knowledge, and pushing is the engine's registry
concern.

### One observer, five callbacks

A run reports everything to **one** `Observer`, supplied by whoever opens the run. There is
no channel to close, no `Events()` getter, no snapshot-plus-delta:

```go
type Observer interface {
    StepStarted(unit, step string, at time.Time)
    StepDone(unit, step string, at time.Time, err error)
    ImageBuilt(unit, image string, at time.Time)
    Published(unit, image, hash string, at time.Time)
    RunDone(unit string, at time.Time, err error)
}
```

Three callbacks are **lifecycle**, two are **output**. The output pair mirrors the
build⊥publish orthogonality
([delivery-verbs-are-orthogonal](../decisions/2026-07-05-delivery-verbs-are-orthogonal.md)):
`ImageBuilt` is the common path — every successful build fires it, and four of the five
commands that build (`build`, `export`, `exec`, `preview`) never publish — while
`Published` fires only on the publish path and is the only place a registry hash exists.
One callback per event kind; nothing is inferred from a shared method with a mode flag.

The kernel stays at these five until a capability actually arrives: the per-step
log-capture callback below is added when capture lands, not front-loaded.

`RunDone` fires **exactly once** per run, whichever way the cursor ends, which makes the
report self-terminating: a consumer needs no out-of-band done signal.

Signatures carry **scalars only, never engine or framework types**. Go interfaces are
structural, so an implementation then needs no platform import at all — that is what lets
the leaf `internal/buildlog` and, later, `srv` satisfy the same methods without importing
the engine or each other.

Everything else is a **fold** of these callbacks: a step's elapsed time is
`StepDone.at − StepStarted.at`; a run's current state, and its scalar outcome, are the
reduction of what it has reported so far. Failure is the `err` on the callback that ends
the step or the run — there is deliberately no separate failure callback, and no
`Event`/`EventKind` type. `StepResult`, `Update`, and `Result` are collapsed into the fold;
`Snapshot`/`Done` are dropped outright — execution moves to a worker that writes to the
database and the webui reads it back, so there is no late-joining live observer to catch up.

#### The accumulator and `TeeObserver`

A run's observer is **never nil**. The engine force-injects an accumulating observer — a
stateful fold of the callbacks — into every run, and that accumulator is the **sole
minter** of the run's scalar outcome (ok/err, image, hash). A caller's observer, when
there is one, is composed alongside it by **`TeeObserver`**: `Tee(obs ...Observer)
Observer` forwards each callback to every child.

**The fold is a type of its own, and the observer that writes it is unexported.** The
accumulator is only a writer; what the rest of the engine wants is the accumulated scalars.
So `outcome` — the three-field fold — is the type `Run` and `BuildResult` hold, and no field
anywhere is typed as a concrete `Observer` implementation. Composition and the fold are
handed over together by one constructor:

```go
func accumulate(caller Observer) (Observer, *outcome)
```

Nothing outside the engine names the accumulator; nothing inside it names one either beyond
that constructor. **Observer-typed fields stay `Observer`** — specializing one to an
implementation is what this shape exists to prevent.

The wrap site is `NewRun` — the fold has to be **run-owned**, because `Run.Result()` mints
its scalars from it. Nil is eliminated **once, there**, so no downstream code carries a
guard: `accumulate` returns the bare accumulator when `caller` is nil and `Tee(acc, caller)`
otherwise.

`Tee`'s contract is **non-nil children only**, and the run's report path has no nil check
at all. A caller that wants nothing simply passes nothing — the fold still happens, because
the result depends on it.

The accumulator is named for accumulating, never `Recorder`: "record" is already taken by
the DB vocabulary (`BuildEvent` records) and by test helpers. `outcome` names the data;
"fold" stays the verb for what the accumulator does to the stream.

Callbacks fire on the **multiplexer's per-unit goroutine**, so an implementation serializes
itself; the engine adds no lock on a caller's behalf.

**Channels arrive with srv's websocket, not before.** A channel-pushing `Observer` is the
adapter that introduces them, and the wire format lives in `srv` — so the engine never owns
a serialization format.

**Log capture rides `Container.Stdout`/`Stderr`, never `WithLogOutput`.** Per-unit
retrieval is incremental and per-step: each step's output is read as that step finishes,
with no re-execution (Dagger caches the walk), landing exactly on the `.Sync()` boundary
`Next` already has — so captured output flushes per step, as a sixth callback added when
capture lands. It demuxes cleanly across units sharing a pooled client, so the client pool
is untouched by log capture. `WithLogOutput` is the Dagger CLI *subprocess's* stderr pipe —
rendered TUI text, never demuxable — and is not a capture path.

**`buildlog` is not build-progress.** `internal/buildlog` is platform's own narration of
what *platform* is doing; what an observer reports is what the *build* is doing. They
coincide only on a machine-local CLI run, and must not be merged: `cmd` owns the
progress-rendering observer and calls `buildlog` as its sink.

**The default CLI shows steps, not Dagger.** Dagger's own `WithLogOutput` TUI is a
debugging firehose gated behind `-v`; at default verbosity a build's visible progress is
exactly the observer's step reports.

### The built container is hidden

The `*dagger.Container` a run produces is **engine-internal**. It is bound to the client
that built it, and it is carried so that steps chain and `Publish` can push it — but
consumers read what the observer reports, not the container. The lone exception is
`preview`, whose post-build tunnel genuinely needs the container handle; it gets it by an
explicit method. That is the one thing a run exposes beyond its report.

### `Run.Result()` — consistent by construction

`Run.Result()` returns a **`BuildResult`**: the join of the injected accumulator's scalar
fold (ok/err, image, hash) with the unit and the live container the run itself owns — the
unit rides the run as its `*framework.BuildUnit`, so the accumulator never restates it. The
two halves are joined at exactly **one site**, because they can only come from there — the
scalars are *derived* from the event stream rather than authored anywhere, and the
container never leaves the run that holds it. There is no hand-packed result assembled at a
call site, so an inconsistent `BuildResult` — a success with no image, a hash from a build
that never published — is unconstructable rather than merely discouraged.

That split is also why the container cannot ride the observer: a `*dagger.Container` is
bound to a client and cannot cross a process boundary, while the scalar half is identical
in-process (a `BuildResult` handed back to `cmd`) and in-database (the same fold, persisted
as `build_events` by the worker — see [platform-server.md](platform-server.md)).

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

## Fan-out lives inside the engine

Fanning out over already-resolved units is **parallel execution, not coordination** — and
parallel execution is exactly what "the engine is capacity" means. So multi-unit fan-out
stays **inside `engine/`**, behind domain verbs. Callers name what they want done, hand
over an observer, and read results:

```go
engine.Build(ctx, cfg, modnames, obs)            // []BuildResult
engine.Publish(ctx, builds...)                   // []PublishResult
engine.BuildAndPublish(ctx, cfg, modnames, obs)  // []PublishResult
```

The generic `multiplexer` is **unexported**. It provides orchestration and synchronization
only and **owns no build method** — it drives the same single-unit path the engine already
has, one `Run` per unit against the one shared `*Engine`. Nothing outside `engine/`
constructs a multiplexer or touches a `Run`.

There is nothing to merge on the reporting side: every unit reports to the same observer
and names itself in each callback, so the fan-in *is* the observer. A per-unit failure
surfaces as the `err` on that unit's `RunDone` and never aborts its siblings.

`cmd` and the srv worker call the **same verbs** and differ only in the observer they pass
— a progress renderer on the CLI, a `build_events` writer in the worker. The worker issues
one verb call per record; the per-unit goroutines live in the engine's multiplexer, never
in the worker. This does not move the scheduling boundary above: *which build runs next*
remains the worker's, and it still never sees a host address.

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
| `Build(ctx, cfg, modnames)`   | `local_arch`, or `publish_arch` when `CI` is true |

Engine entrypoints take `cfg` + module names and construct the units themselves; the arch
rule is engine-internal and unexported, because it is only ever an input to an entrypoint
that is about to build (`CI` is read through fx's own `prompts.CIConfig`, a `config.Bool`
— there is no second `CI` var). Callers never name an arch, and `preview`/`exec` read what
they need off `BuildResult.Unit` after the build rather than holding a unit before it. The
exception is `ls`, a local debugging view that never builds: it assembles its own unit at
`local_arch` directly, as commands with ad-hoc Dagger access are allowed to.

`framework.Units` receives the resolved arch and writes it into each `BuildUnit`
([`framework/unit.go`](../../framework/unit.go)) — the engine then reads the field, never
a call argument. Bare archs become `linux/<arch>`; `auto` tracks `runtime.GOARCH`;
`local_arch` defaults to `auto` and `publish_arch` to `amd64`. The infra `FROM scratch`
manifest image carries no executable, so arch is irrelevant to it.
