# Engine

Status: **design-of-record.** The `engine/` package — the Dagger execution layer that runs
a unit's planned steps and pushes the resulting images. Sits at the tail of the pipeline
([`architecture.md`](architecture.md)): `[]*BuildUnit ─▶ engine.Run ─▶ images`.

## A `*dagger.Client` is a session, not a connection

This is the fact the whole package is shaped around. `dagger.Connect` opens a **session**,
and every container is a handle *into* that session — not a value you hold independently of
it. Sessions are therefore **not fungible**: you cannot take one, use it, hand it back, and
keep using what it produced. Swapping one for another silently invalidates every container
in flight, and closing one invalidates every container built on it.

So the package does **not** pool clients the way `sql.DB` pools connections. A connection
pool is the right abstraction for a fungible resource and the wrong one here; applying it
was what let a session's lifetime get attached to whatever scope happened to be nearby.
Instead there are two things, and only one of them has a lifetime:

- a **stateless roster** ([`engine.go`](../../engine/engine.go)) — which endpoints exist, and
  how to dial one. It caches nothing and holds nothing between calls.
- **`Session`** ([`session.go`](../../engine/session.go)) — the **lifetime**: the span during
  which the containers it produced are usable.

`clients.go` and `runners.go` are **deleted**, not rewritten: the client pool is the
abstraction this design rejects, and the `runners` struct was a config-holder whose only real
job was a test seam. Their content lands in `engine.go`, which is already the package's own
file — that is why there is no `pool.go` and no `roster.go`, and why the `Engine` type going
away does not take `engine.go` with it.

### `Session` — the unit of lifetime

```go
type Session struct {
	ctx   context.Context   // carries both the lifetime and the config
	mu    sync.Mutex
	conns []*dagger.Client
}
```

| Call                        | Role                                                             |
| --------------------------- | ---------------------------------------------------------------- |
| `NewSession(ctx)`           | open a session; dials nothing                                     |
| `Build(ctx, cfg, mods, obs)`| build every matched unit, one run each                            |
| `BuildAndPublish(…, tag, …)`| build and push each image as its run finishes                     |
| `Clean(ctx)`                | prune every fleet engine's local cache (drives `platform clean`)  |
| `Unsafe()`                  | one raw connection for an ad-hoc caller (`ls` only)               |
| `Close()`                   | end every connection it opened, and every container built on one  |

A session opens **as many connections as its work needs** — `connect()` dials one more and
remembers it — and closes them together. One run uses one connection for all its steps,
because a container is bound to the connection that built it; a session driving many runs
dials many, and *that* is what spreads runs across the fleet.

Commands open **one** session and defer `Close`. It is safe for concurrent use, and
`NewRun` does no dialing at all (a connection is taken at the first `Next`), so there is
never a reason to open a second.

🚨 **A session's context is the session's own — no caller's cancellation reaches it.**
Every connection is dialed into `Session.ctx`, a process scope. `Build` gives each unit a
`context.WithTimeout(ctx, unit.Timeout)` and cancels it when the unit ends; that timeout
bounds the unit's **steps** and must never reach a dial, because a unit finishing would
otherwise close a session whose containers the caller still holds. Only `Close` ends a
session.

🚨 **Data may ride in the context; resources may not.** `cfg` travels via
`config.NewContext`/`FromContext` because it is inert and has no lifetime — the roster reads
it with a nil-check fallback to `Configure()`. A `Session` owns connections and has a
`Close`, so it is passed explicitly and never stashed on a context. The two look identical
at a callsite, which is exactly why the rule is written down: an earlier design carried the
engine on the context and type-asserted it back, and that hidden dependency is what made
its lifetime impossible to see.

## Runner discovery

`runners` resolves the configured Dagger endpoints via DNS — no k8s API, no RBAC:

| Config               | Default | Meaning                                                     |
| -------------------- | ------- | ----------------------------------------------------------- |
| `DAGGER_ENGINE`      | unset   | headless-Service DNS of the engine pool                     |
| `DAGGER_ENGINE_PORT` | `1234`  | engine pod port (mirrors `apps/dagger-engine.cue`)          |

`Hosts(ctx)` looks up the DNS name and returns one `tcp://<addr>:<port>` per resolved pod.
It reports **only what it finds**:

- `DAGGER_ENGINE` unset → empty slice, no lookup.
- DNS resolves to nothing → empty slice.
- lookup failure → a real error, surfaced.

Falling back to a local engine is **not** `Hosts`' decision — it reports emptiness and
`Dial` decides, reading an empty roster as "let Dagger auto-provision and reuse the local
engine." So unset `DAGGER_ENGINE` is an explicit operator choice for local, never inferred.

`Dial(ctx)` picks **uniformly at random** among the resolved hosts. Random replaces the old
round-robin cursor: the distribution over a run of picks is the same, and it needs no state
kept between calls — which is what lets the roster stay a roster.

The resolver caches per the DNS record TTL, so a new pod becomes selectable as soon as DNS
reflects it — no restart. Nothing else is remembered: two calls a second apart may
legitimately see different engines as pods come and go, and that is the point.

## `Run` — one unit, one step at a time

`NewRun(sess, unit, caller)` is a **single-unit** run, and it is **engine-internal** — the
domain verbs below open runs, no caller does. It asks the unit's framework for a `Plan`,
then exposes a cursor:

```go
run := NewRun(sess, unit, caller)  // caller may be nil; the run injects its own acc
for run.Next(ctx) { }              // drives exactly one Step per call
```

The **first** `Next` takes the run's connection (`Session.connect()`) and every later step
reuses it: a container is bound to the connection that built it, so one run's steps can
never be spread across the fleet. The session spreads whole *runs*, not steps.

Each `Next` calls `Framework.Execute` for the current `Step` with the previous step's
container, forces the work eagerly with `.Sync()`, and **times** the step across that
boundary. `Next` returns false at the end of the plan or on failure; the error is held on
the run and reported to the observer, not by aborting siblings.

`unit.Timeout` bounds a run's **steps** — `Build` wraps the step loop in
`context.WithTimeout(ctx, unit.Timeout)` and cancels it when the unit ends. It bounds
nothing else: it never reaches a dial, and it does not limit how long the container it
produced stays usable. That is the session's business, not the timeout's.

Clone (repo-prep) and Publish are **engine brackets** around this loop, not framework
steps — cloning is not any stack's build knowledge, and pushing is the engine's registry
concern. Publish being a bracket is load-bearing rather than descriptive: it runs while the
run's connection is still a local variable, which is what lets the registry secret be minted
on the same session as the container it authenticates.

### One observer, five callbacks

A run reports everything to **one** `Observer`, supplied by whoever opens the run. The
contract, the tee and the accumulator are their own package —
[`engine/observer/`](../../engine/observer/), a file each — so `engine` imports the
reporting vocabulary rather than declaring it. There is no channel to close, no `Events()` getter, no
snapshot-plus-delta:

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

#### The accumulator and the tee

A run's observer is **never nil**. The engine force-injects an accumulating observer — a
stateful fold of the callbacks — into every run, and that accumulator is the **sole
minter** of the run's scalar outcome (ok/err, image, hash). A caller's observer, when
there is one, is composed alongside it by a tee: **`Tee(obs ...Observer) Observer`**
forwards each callback to every child.

**`Tee` is the whole surface — the type behind it is unexported**, like the accumulator.
Both are `Observer` implementations, and an implementation is never something a caller
names.

**The fold is a type of its own, and the observer that writes it is unexported.** The
accumulator is only a writer; what the rest of the engine wants is the accumulated scalars.
So `Outcome` — the three-field fold — is the type `Run` and `BuildResult` hold, and no field
anywhere is typed as a concrete `Observer` implementation. Composition and the fold are
handed over together by one constructor:

```go
func Accumulate(caller Observer) (Observer, *Outcome)
```

`Outcome`'s three fields are exported and the accumulator's type is not: the fold crosses
the package line into `engine`, and the writer never does. Nothing outside `observer` names
the accumulator; nothing inside it names one either beyond that constructor.
**Observer-typed fields stay `Observer`** — specializing one to an implementation is what
this shape exists to prevent.

The wrap site is `NewRun` — the fold has to be **run-owned**, because `Run.Result()` mints
its scalars from it. Nil is eliminated **once, there**, so no downstream code carries a
guard: `Accumulate` returns the bare accumulator when `caller` is nil and `Tee(acc, caller)`
otherwise.

`Tee`'s contract is **non-nil children only**, and the run's report path has no nil check
at all. A caller that wants nothing simply passes nothing — the fold still happens, because
the result depends on it.

The accumulator is named for accumulating, never `Recorder`: "record" is already taken by
the DB vocabulary (`BuildEvent` records) and by test helpers. `Outcome` names the data;
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
capture lands. It demuxes cleanly across units sharing a session, so the session layer is
untouched by log capture. `WithLogOutput` is the Dagger CLI *subprocess's* stderr pipe —
rendered TUI text, never demuxable — and is not a capture path.

**`buildlog` is not build-progress.** `internal/buildlog` is platform's own narration of
what *platform* is doing; what an observer reports is what the *build* is doing. They
coincide only on a machine-local CLI run, and must not be merged: `cmd` owns the
progress-rendering observer and calls `buildlog` as its sink.

**The default CLI shows steps, not Dagger.** Dagger's own `WithLogOutput` TUI is a
debugging firehose gated behind `-v`; at default verbosity a build's visible progress is
exactly the observer's step reports.

### The built container is hidden behind one unsafe door

The `*dagger.Container` a run produces is **engine-internal**. It is bound to the connection
that built it, and it is carried so that steps chain and the publish bracket can push it —
but consumers read what the observer reports, not the container.

Three commands genuinely operate on the container and cannot be served by the report:
`preview` (tunnel a port at the built image), `export` (write the image to a file), and
`exec` (run a command or a shell in it). The engine's own verbs for those operations are
**deliberately unbuilt** — chakrit:verbatim "Settle nothing. Keep the same dagger calls
in preview/exec for now" — so the machinery is handed over instead, through exactly one
method whose name is the warning:

```go
func (r BuildResult) UnsafeContainer() *dagger.Container
```

One half, not two: `Export`, `WithExec` and `Publish` are container operations and work from
the container alone. A tunnel is not — `Tunnel` hangs off `*Host`, so it is reached through a
client, and no SDK type hands one back (there is no `*Container`/`*Service` accessor for it).
The container-only path for a tunnel is **`Service.Up`**, which forwards host ports from the
service itself, and that is what `preview` uses. The freeze above is lifted for exactly that
call and nothing else: `exec`'s and `export`'s dagger calls stand unchanged.

The only remaining thing that needs a raw client is minting a `*dagger.Secret`, and the
publish bracket does that where the connection is still in scope. `Session.Unsafe()` exists
for the one caller that wants a connection without building at all (`ls`).

**The container is valid only while its session is open.** That is the whole contract, and
it is why `Close` is deferred by whoever opened the session rather than reached by any
inner scope.

`Unsafe` is the whole point of the name — a caller reaching past the engine's report says so
at the callsite, and a reviewer greps one word to find every such caller. It is **not**
`Must`, which the engine lexicon spends on panic-style fetches.

So `BuildResult` carries no exported dagger field and the engine exposes no other dagger
handle. The three callers do express container operations, which
[§No dagger verbs outside `engine/`](#no-dagger-verbs-outside-engine) otherwise forbids:
they are the known, bounded set of that, they announce it in one word, and no new caller
joins them. When the verbs land, they replace these callers and the door closes —
reconcile this section then, never the code before then.

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
| which runner executes it | engine | `Hosts()`, uniform choice at dial |

**The worker never sees a host address.** If one leaks upward the boundary is gone and
two schedulers begin fighting over the same capacity.

### No dagger verbs outside `engine/`

Callers must never *express container operations*: no `WithExec`, no `WithDirectory`, no
`Sync`. Each of those is an execution decision, and authoring one outside the engine moves
part of the build definition out of the layer that owns it. The tell is a dagger
constructor appearing anywhere outside `engine/`.

This is a rule about **logic, not types**. Go type inference means `c := sess.Build(…)`
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
sess.Build(ctx, cfg, modnames, obs)                 // []BuildResult
sess.BuildAndPublish(ctx, cfg, modnames, tag, obs)  // []PublishResult
```

**There is no standalone `Publish` verb.** Its only possible argument is a `BuildResult`,
which only `Run.Result` can mint, so holding one means you already built in that session —
there is no reachable state where publishing without a build makes sense. "Publish something
built earlier" is a registry-to-registry copy: no container, no engine, not this package.
Publishing is a bracket inside the run, not a second pass over results.

The generic `multiplexer` is **unexported**. It provides orchestration and synchronization
only and **owns no build method** — it drives the same single-unit path the engine already
has, one `Run` per unit against the one open `*Session`. Nothing outside `engine/`
constructs a multiplexer or touches a `Run`.

There is nothing to merge on the reporting side: every unit reports to the same observer
and names itself in each callback, so the fan-in *is* the observer. A per-unit failure
surfaces as the `err` on that unit's `RunDone` and never aborts its siblings.

`cmd` and the srv worker call the **same verbs** and differ only in the observer they pass
— a progress renderer on the CLI, a `build_events` writer in the worker. The worker issues
one verb call per record; the per-unit goroutines live in the engine's multiplexer, never
in the worker. This does not move the scheduling boundary above: *which build runs next*
remains the worker's, and it still never sees a host address.

## Publishing

The publish bracket pushes a successfully-built container on the connection that built it,
so the registry secret is minted by the same session that owns the container, and logs the
image via `buildlog.Image`. `publish` composes the ordinary path — build the units at the
publish arch, suffix each `ImageName` with the release tag, run, then push — and the
`[]BuildAttempt` records of what shipped are assembled by `srv`
([platform-server.md](platform-server.md)), not by the engine.

`release` (cut a tag) and `publish` (build + push) are orthogonal — neither implies the
other, and there is no `deploy` verb. See
[delivery-verbs-are-orthogonal](../decisions/2026-07-05-delivery-verbs-are-orthogonal.md)
for the full rationale.

## Registry credentials

The publish bracket reads three fx env-config values off the session's config source:

| Config              | Role                          |
| ------------------- | ----------------------------- |
| `REGISTRY`          | registry host for auth        |
| `REGISTRY_USERNAME` | registry user                 |
| `REGISTRY_PASSWORD` | registry secret (set via `client.SetSecret`, never inlined) |

When `REGISTRY_USERNAME` is empty, the bracket skips `WithRegistryAuth` entirely — Dagger
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

| Entrypoint                        | Arch                                    |
| --------------------------------- | --------------------------------------- |
| `sess.BuildAndPublish`            | `publish_arch` — pushing *is* the answer |
| `sess.Build(ctx, cfg, modnames)`  | `local_arch`, or `publish_arch` when `CI` is true |

Session entrypoints take `cfg` + module names and construct the units themselves; the arch
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
