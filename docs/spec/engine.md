# Engine

Status: **design-of-record.** The `engine/` package is the reusable source and build driver
behind both CLI and server adapters. Its source verb materializes a remote revision as a
managed local worktree; its session verbs load configuration, interpret modules, place work
on Dagger runners, execute steps, and optionally publish. It knows nothing about the web
application that invokes it ([`architecture.md`](architecture.md)).

The source-input and paired-observer contracts below are the **Phase 2 implementation
target**. All observer consumers convert together with module execution and lifecycle
reads; a partially converted interface is not a usable candidate. The server's later
publication-policy rollout does not change these engine contracts.

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
	ctx       context.Context // carries both the lifetime and the config
	mu        sync.Mutex
	conns     []*dagger.Client
	worktrees []*Worktree
}
```

| Call                           | Role                                                               |
| ------------------------------ | ------------------------------------------------------------------ |
| `NewSession(ctx)`              | open a session; dials nothing                                      |
| `Build(ctx, input, mods, obs)` | build every selected module, one operation each                    |
| `BuildAndPublish(…, tag, …)`   | build and push each image as its run finishes                      |
| `Clean(ctx)`                   | prune every fleet engine's local cache (drives `platform clean`)   |
| `Unsafe()`                     | one raw connection for an ad-hoc caller (`ls` only)                |
| `Close()`                      | close connections, then release operation-owned worktrees          |

A session opens **as many connections as its work needs** — `connect()` dials one more and
remembers it — and closes them together. One run uses one connection for all its steps,
because a container is bound to the connection that built it; a session driving many runs
dials many, and *that* is what spreads runs across the fleet.

Commands open **one** session and defer `Close`. It is safe for concurrent use, and
module preparation dials inside its configuration phase, so there is never a reason to
open a second session for a later phase of the same module.

Repository checkout is independent of a Dagger session:

```go
Checkout(ctx, source Source) (*Worktree, error)
```

`Source` carries only driver-level facts: URL, credential, and revision. `Worktree` exposes
its directory and resolved commit SHA, and `Close(ctx)` removes it. Cache identity,
credential injection, mirror synchronization, revision resolution, unique worktree paths,
and pruning stay private. A hypothetical checkout CLI could call this surface directly;
that is the boundary test, not a command this design adds.

### Build inputs and lifecycle ownership

The existing `Build` and `BuildAndPublish` verbs accept an `Input` in place of an
already-loaded model. `Input` is a closed choice of two value variants:

| Input      | Carries                                      | Configuration source                   |
| ---------- | -------------------------------------------- | -------------------------------------- |
| `Local`    | `ConfigPath`, the selected config-file path  | that file, resolved by CLI preflight   |
| `Source`   | remote URL, revision, fetch credentials      | `platform.toml` at the checkout root   |

There is no local/remote boolean or set of nullable companion fields. `Source` remains
the same generic input accepted by standalone `Checkout`; it carries no build record,
manifest id, publication policy, job, or server identity. `conf` owns loading an explicit
file, its defaults, environment overrides, and relative-path binding. Engine composes that
existing owner with checkout and execution; neither it nor the worker copies the parser.

Module selection precedes the per-module lifecycle. For `Local`, an empty name list
retains the CLI's all-modules convention: engine reads the selected file in a selection
preflight to discover names. A CLI may already have read that file to resolve its release
tag or confirmation prompt; it still passes the file path, not a pre-interpreted unit or
a model for execution. A failed selection preflight returns an invocation error and starts
no module callbacks, because no selected module identity exists yet. An empty manifest
returns `ErrNoJobs`. Explicit names need no discovery pass.

For `Source`, the name list must be explicit and nonempty. The server supplies exactly
its one persisted selected module name. Engine does not discover more work from the remote
checkout or silently interpret an empty server selection as all modules.

For each selected name, engine creates the accumulator and starts the outer operation
before doing any source work. A remote input runs `Checkout` inside the clone pair; a
local input starts configuration directly. The remote loader names the exact
`worktree.Dir/platform.toml` file; it never searches parent directories for a substitute.
Inside the config pair, engine loads the selected file, interprets exactly that module,
validates its plan and any supplied publication credentials, and dials its connection.
A missing selected name is a configuration failure for that name. Local configuration is
loaded in this observed phase even when discovery preflight already read it; the preflight
chooses names and is not an unreported substitute for the configuration phase.

The outer operation then drives the step cursor and optional publication and reports
`RunDone` once. The cursor cannot report an early whole-operation completion. All returned
phase failures pass through this owner, including failures before a `BuildUnit` or a
container exists. A preparation failure has an observer outcome and a returned error;
it does not fabricate an interpreted unit or a usable container.

The worker supplies source credentials, selected name, and its build-versus-publish
decision. It never calls observer methods to manufacture missing engine events. A failure
to obtain the request's prerequisites, such as a GitHub installation token or a database
read, is a job error before engine invocation; the claim remains observable, and the job
mechanism is not authority to execute that claimed module again.

Standalone `Checkout` still transfers worktree ownership to its caller. When a session
build verb performs checkout, the session retains that worktree until `Close`, which
closes connections before releasing worktrees and returns cleanup errors. This keeps
source files available to the live containers used by `preview`, `exec`, and `export`.
Checkout itself still requires no Dagger session, and no adapter manages its cache paths.

🚨 **A command that holds a session returns its errors; it never exits from inside.**
`os.Exit` runs no deferred function, so a command that reaches `termlog.Fatalln` on a
failure path abandons every connection the session opened — the one case where the
deferred `Close` above is a promise the code does not keep, and it is the failure paths
where an engine is most likely to be left holding state. Session-holding commands are
therefore Cobra `RunE`: they return, `main` reports and exits once, and `Close` runs on
the way out. Commands that open no session keep `Run` — the rule follows the session, not
tidiness.

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

The roster resolves the configured Dagger endpoints via DNS — no k8s API, no RBAC:

| Config               | Default | Meaning                                            |
|----------------------|---------|----------------------------------------------------|
| `DAGGER_ENGINE`      | unset   | headless-Service DNS of the engine pool            |
| `DAGGER_ENGINE_PORT` | `1234`  | engine pod port (mirrors `apps/dagger-engine.cue`) |

`DAGGER_ENGINE` is the one source everywhere — CLI, srv, and worker read the
env directly, and k8s DNS itself spreads connections across the pool's pods.
No engine binding is stored server-side ([installation.md](installation.md),
the env contract).

`hosts(ctx)` looks up the DNS name and returns one `tcp://<addr>:<port>` per resolved pod.
It reports **only what it finds**:

- `DAGGER_ENGINE` unset → empty slice, no lookup.
- DNS resolves to nothing → empty slice.
- lookup failure → a real error, surfaced.

Falling back to a local engine is **not** `hosts`' decision — it reports emptiness and
`dial` decides, reading an empty roster as "let Dagger auto-provision and reuse the local
engine." So unset `DAGGER_ENGINE` is an explicit operator choice for local, never inferred.

**The roster is unexported in full.** `hosts` and `dial` are reached only through
`Session`. Nothing outside `engine/` needs an endpoint list, and exporting one invites a
caller to dial around the driver that owns execution.

**The one endpoint fact that does cross the boundary is the assigned one.** A run reports
`ConfigDone(unit, engine, at, nil)` after its connection succeeds and before its first
step starts. The scalar `engine` is `host:port` for a configured endpoint, or the literal
`local` when the SDK manages the local engine. `local` names that selection mode, not a
discovered pod or container identity; no SDK internals are inspected to invent one. It is
distinct from a failed assignment, which carries an empty string. The server stores this
observation as the module's `engine_host` and `engine_assigned_at`, without duplicating
the endpoint across events ([platform-server.md](platform-server.md), the build tables).
Configured endpoints permit instance attribution; `local` reports only SDK-managed
execution on the claiming worker. Neither value exposes a roster or dialing surface.

`dial(ctx)` picks **uniformly at random** among the resolved hosts. Random replaces the
old round-robin cursor: the distribution over a run of picks is the same, and it needs
no state kept between calls — which is what lets the roster stay a roster.

The resolver caches per the DNS record TTL, so a new pod becomes selectable as soon as DNS
reflects it — no restart. Nothing else is remembered: two calls a second apart may
legitimately see different engines as pods come and go, and that is the point.

## `Run` — one unit, one step at a time

`Run` is the **engine-internal** step cursor for one interpreted unit. The domain verbs
own the surrounding module operation: source preparation, configuration, the step cursor,
and optional publication. Callers construct neither the operation nor the cursor.
Configuration obtains the framework's `Plan` and the connection before the cursor starts:

```go
for run.Next(ctx) { } // drives exactly one framework Step per call
```

Every `Next` reports `StepStart` and executes the step on the already-connected client.
Every step reuses that connection: a container is bound to the connection that built it,
so one module's steps can never be spread across the fleet. A failed dial ends with
`ConfigDone(error)` and `RunDone(error)`, with no assignment or step start.

Each `Next` calls `Framework.Execute` for the current `Step` with the previous step's
container, forces the work eagerly with `.Sync()`, and **times** the step across that
boundary. `Next` returns false at the end of the plan or on failure; the error is held on
the run and reported to the observer, not by aborting siblings.

`unit.Timeout` bounds a run's **steps** — `Build` wraps the step loop in
`context.WithTimeout(ctx, unit.Timeout)` and cancels it when the unit ends. It bounds
nothing else: it never reaches a dial, and it does not limit how long the container it
produced stays usable. That is the session's business, not the timeout's.

🚨 **An empty plan is a failed run, not an empty success.** A framework that returns no
steps has produced nothing, so a run over it must not report an image or hand back a
result claiming one — `Result` says a success with no container cannot be constructed, and
that claim is only true if the empty plan is rejected. The run fails at open with
`ErrEmptyPlan` and executes nothing. This is `unknownStep`'s twin: that one is unreachable
while `Plan` and `Execute` agree, this one while `Plan` returns anything at all, and both
stay loud precisely because a silent version of either is a build stage that vanished.

Repository checkout and publishing are engine capabilities; neither is a framework step.
They remain independent because a worktree and a Dagger session have unrelated lifetimes.
Publish being an engine bracket is load-bearing: it runs while the run's connection is
still a local variable, so the registry secret belongs to the same session as the container
it authenticates.

### Repository preparation

`Checkout` materializes a `Source` into a local `Worktree`. It keeps one full bare mirror
per credential-free URL, fetches it under a per-mirror lock, resolves the requested commit,
and creates a uniquely named, independently removable worktree. It uses plain `git`,
because config loading, in-process CUE rendering, and Dagger `host.Directory` all need the
same local filesystem tree.

The clone token is injected into the fetch URL for that invocation only. The stored remote
remains credential-free. Clones are never shallow because repositories may use history-
dependent operations such as `git subtree`; the mirror makes later full fetches cheap.

```
<cache>/git/<opaque-url-key>.git  <- bare mirror
<cache>/work/<unique-id>/         <- independent worktree
```

`Worktree.Close` owns removal and mirror pruning. Its owner must observe and report cleanup
errors after the operation; cleanup does not manufacture another `RunDone`.
URL keys and mirror/worktree manipulation stay inside `engine`; callers receive no cache
path or worktree identifier knob.

`CHECKOUT_CACHE` sets the checkout cache root. Unset, `Checkout` uses the operating
system's per-user cache directory plus `platform`; the worker deployment sets it to
`/var/cache/platform`. This is engine configuration, not a `Source` field, so adapters do
not choose storage per operation.

### One observer, paired phases

An engine operation reports everything to **one** `Observer`, supplied by its caller. The
contract, the tee and the accumulator are their own package —
[`engine/observer/`](../../engine/observer/), a file each — so `engine` imports the
reporting vocabulary rather than declaring it. There is no channel to close, no `Events()` getter, no
snapshot-plus-delta:

```go
type Observer interface {
    RunStart(unit string, at time.Time)
    CloneStart(unit string, at time.Time)
    CloneDone(unit string, at time.Time, err error)
    ConfigStart(unit string, at time.Time)
    ConfigDone(unit, engine string, at time.Time, err error)
    StepStart(unit, step string, at time.Time)
    StepOutput(unit, step string, at time.Time, stdout, stderr string)
    StepDone(unit, step string, at time.Time, err error)
    PublishStart(unit string, at time.Time)
    PublishDone(unit string, at time.Time, err error)
    RunDone(unit, image, hash string, at time.Time, err error)
}
```

The eleven callbacks describe one module operation, with the same nonempty module name
throughout:

```text
RunStart
  CloneStart -> CloneDone                         # remote source only
  ConfigStart -> ConfigDone                       # includes engine assignment
  StepStart -> optional StepOutput -> StepDone    # for each framework step
  PublishStart -> PublishDone                     # requested publication, after steps succeed
RunDone
```

Engine owns every callback. `RunStart` precedes checkout; `ConfigStart` precedes loading
the selected configuration. Config covers parsing, module interpretation, plan validation,
and connection. Successful `ConfigDone` carries the endpoint actually connected to;
failed `ConfigDone` carries an empty endpoint. No step starts before configuration succeeds.

Every started phase closes once on ordinary success or returned failure. Failure closes
that phase with its error, then closes the outer operation with `RunDone(error)` without
starting later phases. Clone failure therefore produces `CloneDone(error)` followed by
`RunDone(error)`. Publish failure produces `PublishDone(error)` before `RunDone(error)`.
Process termination can interrupt reporting; pairing does not promise a callback after a
process has died. A local operation emits no clone callbacks; a build-only operation
emits no publish callbacks.

`RunDone` carries the scalar result, so no separate image-output callback remains. Its
`image` is empty until all framework steps succeed. A build-only success carries the
built image reference and an empty `hash`; publish success carries the published reference
and registry hash. A publish failure retains the built image reference, an empty hash,
and the error. `PublishDone` reports phase completion, not a second copy of those outputs.
An empty plan or failed build cannot report an image. The first phase failure remains the
operation's error; a later completion never erases it.

Signatures carry **scalars only, never engine or framework types**. Go interfaces are
structural, so an implementation then needs no platform import at all — that is what lets
the leaf `internal/termlog` and, later, `srv` satisfy the same methods without importing
the engine or each other.

Everything else is a **fold** of these callbacks: a step's elapsed time is
`StepDone.at − StepStart.at`; a run's current state, and its scalar outcome, are the
reduction of what it has reported so far. Failure is the `err` on the callback that ends
the clone, config, step, publish, or run — there is deliberately no separate failure
callback, and no `Event`/`EventKind` type. `StepResult`, `Update`, and `Result` are
collapsed into the fold;
`Snapshot`/`Done` are dropped outright — execution moves to a worker that writes to the
database and the webui reads it back, so there is no late-joining live observer to catch up.

#### The accumulator and the tee

An operation's observer is **never nil**. The engine entrypoint force-injects an
accumulating observer — a stateful fold of the callbacks — before reporting its first
lifecycle callback. That accumulator is the **sole minter** of the scalar outcome
(ok/err, image, hash). A caller's observer, when there is one, is composed alongside it
by a tee: **`Tee(obs ...Observer) Observer`** forwards each callback to every child.

**`Tee` is the whole surface — the type behind it is unexported**, like the accumulator.
Both are `Observer` implementations, and an implementation is never something a caller
names.

**The fold is a type of its own, and the observer that writes it is unexported.** The
accumulator is only a writer; what the rest of the engine wants is the accumulated scalars.
So `Outcome` — the three-field fold — is the type the operation and `BuildResult` hold, and no
field anywhere is typed as a concrete `Observer` implementation. Composition and the fold
are handed over together by one constructor:

```go
func Accumulate(caller Observer) (Observer, *Outcome)
```

`Outcome`'s three fields are exported and the accumulator's type is not: the fold crosses
the package line into `engine`, and the writer never does. Nothing outside `observer` names
the accumulator; nothing inside it names one either beyond that constructor.
**Observer-typed fields stay `Observer`** — specializing one to an implementation is what
this shape exists to prevent.

The wrap site is the module-operation boundary, **before `RunStart`**, rather than the
later step-cursor constructor. The composed observer and fold cover preparation failures
as well as steps and publication. `RunDone` supplies the image/hash and terminal error;
the result reads those scalars from that sole fold. Nil is eliminated once at that
boundary: `Accumulate` returns the bare accumulator when `caller` is nil and
`Tee(acc, caller)` otherwise.

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
`Next` already has — so captured output flushes per step. It demuxes cleanly across units
sharing a session, so the session layer is untouched by log capture. `WithLogOutput` is the
Dagger CLI *subprocess's* stderr pipe — rendered TUI text, never demuxable — and is not a
capture path.

Capture is no longer deferred: `srv` persists a step's output in `build_events`
([platform-server.md](platform-server.md)), so a build whose logs only ever reached a
terminal would be unreadable in the webui — which is the whole point of the server.

```go
StepOutput(unit, step string, at time.Time, stdout, stderr string)
```

It fires on the same `.Sync()` boundary as `StepDone`, before it, so a consumer that only
stores the terminal row still has the output in hand.

**`termlog` is not build-progress.** `internal/termlog` is platform's own narration of what
*platform* is doing, rendered for the operator's terminal; what an observer reports is what
the *build* is doing. They coincide only on a machine-local CLI run, and must not be merged:
`cmd` owns the progress-rendering observer and calls `termlog` as its sink, while the
server's build job implements the same interface and writes `build_events` instead. The
package is named for the terminal it writes to precisely because the older name, `buildlog`,
kept inviting the merge.

**Every line platform emits goes through a typed constructor.** `termlog` exposes one
function per kind of output — `Event(obj, action)`, `Config`, `Error`, `Git`, `File`,
`Image`, `HTTPServing` — and output is grouped by construction rather than filtered after
the fact. A new kind of output is a new constructor, never an ad-hoc `Logger()` call at the
emit site. `Event` is the general shape: something happened to something, so it takes the
object and the action (`termlog.Event("web/build", "started")`). The unit is deliberately
not a field of its own — `termlog` knows nothing of what the object is.

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

### Operation results — consistent by construction

Both build verbs return **`BuildResult`** values after `RunDone`: the join of the injected
accumulator's scalar fold (ok/err, image, hash) with the interpreted unit and the live
container the step cursor owns. The unit stays a `*framework.BuildUnit`, so the accumulator
never restates it. The two halves are joined at exactly **one site** — the
scalars are *derived* from the event stream rather than authored anywhere, and the
container remains owned by the cursor. There is no hand-packed result assembled at a
call site, so an inconsistent `BuildResult` — a success with no image, a hash from a build
that never published — is unconstructable rather than merely discouraged. Preparation
failures return errors without constructing container-bearing results; their callbacks
still close the operation. A multi-module invocation retains successful sibling results
and joins failures after every selected module finishes.

The result carries its interpreted `Unit` and `Err`, exposes the accumulated scalars through
`Image() string` and `Hash() string`, and retains the existing `UnsafeContainer` boundary.
There is no separate `PublishResult` with copied image/hash fields: both verbs consume
the same completed outcome. Only the publication path can fill the hash. Results cannot
continue publishing or emit more callbacks after being returned.

That split is also why the container cannot ride the observer: a `*dagger.Container` is
bound to a client and cannot cross a process boundary, while the scalar half is identical
in-process (a `BuildResult` handed back to `cmd`) and in-database (the same fold, persisted
as `build_events` by the worker — see [platform-server.md](platform-server.md)).

## The driver and runner boundary

The `engine` package is a reusable driver; a Dagger **runner** is execution capacity.
Checkout knows remote source mechanics. Build verbs know selected modules and optional
publish intent. Neither knows repositories as web resources, queues, jobs, users,
installations, or why an adapter requested the operation. A runner receives a resolved
`BuildUnit` and knows even less: only how to execute it.

The driver owns finding runner capacity and executing against it. srv owns deciding which
record runs next, supplying authorized request facts, and recording the report.

### Two scheduling decisions, two layers

Scheduling splits in two and the halves must not meet:

| Decision                 | Owner  | Inputs                              |
|--------------------------|--------|-------------------------------------|
| which build runs next    | worker | pending records, concurrency policy |
| which runner executes it | engine | the roster, uniform choice at dial  |

**The worker observes the assigned host; it never chooses one.** Successful `ConfigDone` reports
the engine's completed choice so the worker can store attribution. The worker receives no
roster, makes no selection, and has no dialing surface; otherwise two schedulers would
fight over the same capacity.

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

Fanning out over selected module names is **parallel execution, not coordination** — and
parallel execution is exactly what "the engine is capacity" means. So multi-unit fan-out
stays **inside `engine/`**, behind domain verbs. Callers name what they want done, hand
over an observer, and read results:

```go
sess.Build(ctx, input, modnames, obs)                 // ([]BuildResult, error)
sess.BuildAndPublish(ctx, input, modnames, tag, obs)  // ([]BuildResult, error)
```

**There is no standalone `Publish` verb.** Its only possible argument is a `BuildResult`,
which only the completed operation can mint, so holding one means you already built in
that session. There is no reachable state where publishing without a build makes sense.
"Publish something built earlier" is a registry-to-registry copy: no container, no engine,
not this package.
Publishing is inside the module operation, before `RunDone`, not a second pass over results.

The generic `multiplexer` is **unexported**. It provides orchestration and synchronization
only and **owns no build method** — it drives one module operation per selected name
against the one open `*Session`. Nothing outside `engine/`
constructs a multiplexer or touches a `Run`.

There is nothing to merge on the reporting side: every unit reports to the same observer
and names itself in each callback, so the fan-in *is* the observer. A per-unit failure
surfaces as the `err` on that unit's `RunDone` and never aborts its siblings.

`cmd` and the srv worker consume the same driver through different adapters. CLI build
commands resolve a local config path and call session verbs with a progress-rendering
observer. The worker translates its persisted source facts into `Source` and calls
`Build` or `BuildAndPublish` with one selected name and a `build_events` observer. Engine
owns the checkout/config/step/publish composition for both inputs. Neither adapter drives
a `Run`, emits engine callbacks, or sees the runner roster.

## Publishing

The publish bracket pushes a successfully-built container on the connection that built it,
so the registry secret is minted by the same session that owns the container, and logs the
image via `termlog.Image`. `BuildAndPublish` composes the ordinary path — build the units
at the publish arch, suffix each `ImageName` with the caller-supplied image tag, run, then
push — and the per-module result folds of what shipped are assembled by `srv`
([platform-server.md](platform-server.md)), not by the engine.

The engine owns neither invocation nor cadence. Local `./platform publish` and the server
worker call this shared capability from different contexts under different policies; see
[`execution-modes.md`](execution-modes.md). Release naming remains outside the engine.

## Registry credentials

The publish bracket reads three fx env-config values off the session's config source:

| Config              | Role                                                        |
|---------------------|-------------------------------------------------------------|
| `REGISTRY`          | registry host for auth                                      |
| `REGISTRY_USERNAME` | registry user                                               |
| `REGISTRY_PASSWORD` | registry secret (set via `client.SetSecret`, never inlined) |

For an operation that publishes, a nonempty `REGISTRY_USERNAME` requires a nonempty
password and a registry host matching the configured module image. Missing or mismatched
credentials fail configuration before steps or publication. An absent saved server token
therefore reaches `ConfigDone(error)` and `RunDone(error)` through this same validation;
the server still supplies its registry and installation username. A build-only operation
does not read or validate publication credentials.

When `REGISTRY_USERNAME` is empty, the bracket skips `WithRegistryAuth` entirely — Dagger
then pushes with the **local docker credentials** (osxkeychain). That is the local-publish
path: a `platform publish` on a laptop needs no `REGISTRY_USERNAME`/`PASSWORD`, only a
docker login to ghcr. The env creds are for a server driver with no local docker config —
the server's worker feeds all three per build from the wizard-saved registry token and
the installation record ([installation.md](installation.md), "The registry token"); the
bracket itself is unchanged and never reads settings.

## Arch targets

**The arch question is not where you are standing, it is whether the image outlives the
box that built it.** A build whose output is pushed to a registry runs somewhere else and
must carry that somewhere's arch; a build-and-discard runs here and takes the host arch
for speed. "Local vs publish" reads as a location and breaks the moment a non-publish
verb runs on a build-server, where nothing is local — so there is no `Target` type and no
declared intent. There is a resolved arch string and the engine entrypoint that resolves
it:

| Entrypoint                              | Arch                                                |
| --------------------------------------- | --------------------------------------------------- |
| `sess.BuildAndPublish`                  | `publish_arch` — pushing *is* the answer            |
| `sess.Build(ctx, input, modnames, obs)` | `local_arch`, or `publish_arch` when `CI` is true   |

Session entrypoints load configuration and construct the selected units themselves; the arch
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
