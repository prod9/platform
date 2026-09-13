# Build lifecycle through the observer

**Historical note — superseded.** This records the pre-Phase-2 investigation and its
unadopted proposal. References and uses of "current" below refer to that investigation,
not the present tree. The implemented contracts are in
[engine.md](../docs/spec/engine.md) and
[platform-server.md](../docs/spec/platform-server.md); this note authorizes no work.

This explanation describes the implementation inspected at `c1bfef2`, then the changes
proposed by `docs/scratch/2026-09-13-module-job-plan.md`. It is for reviewing that plan;
the proposed lifecycle is not a claim about current behavior.

## What the observer is

`Observer` is a Go interface with six reporting callbacks. Engine calls those methods
directly as work progresses; the callbacks carry unit names, step names, timestamps,
errors, image references, and output. They return no error and do not control execution.
The observer is not itself a queue, database record, or separate background process.
See `engine/observer/observer.go:21` and `engine/run.go:250`.

Each engine `Run` represents one interpreted build unit. `NewRun` creates an accumulator
and combines it with the caller's observer through `Tee`:

```text
Run emits a callback
  -> accumulator updates its in-memory Outcome (image, hash, first error)
  -> caller's observer receives the same callback
       CLI: terminal progress
       server worker: transcription to database records
```

The accumulator exists even if the caller supplies no observer. The engine constructs
its result from that outcome plus the live container. A shared caller observer can receive
callbacks from multiple units running concurrently; each callback names its unit.
Sources: `engine/run.go:76`, `:186`; `engine/observer/accumulate.go:23`;
`engine/observer/tee.go:11`; `engine/session.go:72`; `cmd/cmd.go:42`.

## Current lifecycle, in execution order

### Before engine Run creation

The server worker loads the `Build` record and constructs a transcriber bound to its ID.
It obtains credentials, checks out the repository, loads configuration, and prepares
publication credentials when needed. None of those operations currently emits an engine
observer callback. The session then interprets the configuration into units before
constructing their `Run` objects; interpretation errors also precede observer reporting.
Sources: `srv/builds/run_build.go:44`, `:64`; `engine/session.go:63`.

If preparation fails, the worker invokes `scribe.RunDone` itself, with an empty unit
name and the error. This is how an otherwise silent failure reaches the database today;
there is no corresponding start event. Source: `srv/builds/run_build.go:50`.

### Each framework step

For every unit, `Run.Next` follows this sequence:

```text
StepStarted(unit, step)
  execute the step, including connecting on the first step
  force the returned container's work with Sync
  collect stdout/stderr
StepOutput(unit, step, stdout, stderr)   # only when output is nonempty
StepDone(unit, step, error-or-nil)
```

Output is collected after step execution; this is not live line-by-line log streaming.
For a Dagger execution failure, captured output comes from the error. The first engine
connection is currently inside the first step's measured span; there is no assignment
callback. Sources: `engine/run.go:21`, `:89`, `:124`, `:150`.

A failed step stops that unit's step loop. A successful step advances to the next step.
The session drives separate units through its existing fan-out, so one unit's step
failure does not stop sibling loops. Sources: `engine/run.go:110`;
`engine/session.go:72`, `:142`.

### Finishing a build-only run

```text
successful steps -> ImageBuilt(unit, image) -> RunDone(unit, nil)
failed step      -> StepDone(unit, step, error) -> RunDone(unit, error)
empty plan       -> RunDone(unit, error)
```

`finish` guards against reporting completion twice. A failed run emits no `ImageBuilt`;
an empty plan fails before any step is started. These guarantees apply after a `Run` has
been constructed. Sources: `engine/run.go:76`, `:163`.

### Publication currently happens after RunDone

The implemented publish path is:

```text
steps -> ImageBuilt -> RunDone(nil) -> attempt registry push
                                       success -> Published(image, hash)
                                       failure -> returned error, no callback
```

`Session.BuildAndPublish` first finishes `runUnit`, then calls `publish` on that run.
`publish` skips failed builds, applies credentials, and pushes the image. On success it
emits `Published`; on failure it sets the returned result's error without notifying the
observer. Sources: `engine/session.go:99`; `engine/run.go:220`.

This creates a concrete reporting gap: the server worker accepts a returned engine error
only while the transcriber is silent. A publish failure follows already-emitted build
events, so it does not get persisted by that fallback. The observer stream therefore
cannot describe that publication failure, despite the engine returning it to its caller.
Sources: `srv/builds/run_build.go:103`; `engine/run.go:232`.

## How callbacks become records

The server transcriber supplies the persistence behavior; engine has no database access.

| Callback        | Current persistence behavior                                |
|-----------------|-------------------------------------------------------------|
| StepStarted     | Append `step_started` with build ID, unit, step, and time   |
| StepOutput      | Hold stdout/stderr in memory, keyed by unit and step        |
| StepDone        | Append `step_done` with error and the held output           |
| ImageBuilt      | Append `image_built` with the image reference               |
| Published       | Append `published` with image reference and registry hash   |
| RunDone         | Append `run_done` with the terminal error, if any           |

`AppendEvent` owns the actual insert. The transcriber remembers its first persistence
error and exposes it through `Err()` because callback methods cannot return errors;
later callbacks still attempt their writes. The worker returns this error as a job
failure. Sources: `srv/builds/transcribe.go:53`, `:98`; `srv/builds/events.go:58`;
`srv/builds/run_build.go:58`.

API reads derive status/results and steps from the stored events. Currently events are
attached to a build with a string unit name; the planned model attaches them to a
persisted build-module identity. Neither model requires storing each computed step as a
separate row. Sources: `srv/builds/events.go:28`, `attempt.go:32`, `step.go:23`;
`docs/spec/platform-server.md:592`.

## What the plan changes

The reporting mechanism remains direct callbacks, an accumulator, and a caller observer.
The extension makes preparation and placement visible and associates execution with
one persisted module record.

| Proposed callback   | Meaning                                                 |
|---------------------|---------------------------------------------------------|
| CloneStarted        | Repository materialization has begun                    |
| CloneDone           | Materialization ended, with success or error            |
| RunStarted          | Build phase began, before config loading/interpretation |
| EngineAssigned      | Connection succeeded to the selected engine endpoint    |

These four callbacks are in the intended spec but absent from the current interface.
Source: `docs/spec/engine.md:218`; `engine/observer/observer.go:21`.

The intended server lifecycle is:

```text
build/module records created                 # domain action, not observer
module job atomically claims its module      # domain action, not observer

CloneStarted
CloneDone
RunStarted
  configuration loading and interpretation
  engine selection and connection
EngineAssigned
  [StepStarted -> optional StepOutput -> StepDone] for each step
ImageBuilt
Published                                    # only for a successful publish
RunDone
```

The server transcriber will attach events to `build_module_id`. `EngineAssigned` updates
that module's engine host and assignment timestamp rather than appending another event
kind. Step output still rides the `step_done` row. Module state comes from the module
record and its events; overall build state includes every selected module, including
ones that have emitted nothing. `Build` remains the execution identity, with no separate
`BuildAttempt`. Sources: `docs/spec/platform-server.md:479`, `:535`, `:549`, `:592`;
`docs/scratch/2026-09-13-module-job-plan.md`, "Settled model".

For remote work, a clone failure ends at `CloneDone(error)` with no run callbacks.
Failures after `RunStarted` must reach one `RunDone(error)`, even before the first step.
Local-model builds omit the clone callbacks. A crashed process cannot guarantee a final
callback; the claim and last recorded event remain the available evidence, and automatic
retry is not introduced. Sources: `docs/spec/engine.md:233`, `:254`;
`docs/spec/platform-server.md:494`.

## Boundaries still requiring agreement

The plan has not yet settled the API that carries one observer across checkout and
configuration loading: the spec assigns those lifecycle reports to engine while also
describing the worker as composing checkout, config loading, and session calls
(`docs/spec/engine.md:233`, `:487`). That ownership must be made explicit before coding.

Publication completion also needs explicit reconciliation: the server spec places
`Published` before `RunDone`, while implementation places it afterward and engine prose
describes `RunDone` as ending the build phase. The full-operation sequence above is the
server spec's target, not a silently approved change of meaning. Whatever boundary is
chosen must make publication failure observable; adding four callbacks alone does not
repair this gap. Sources: `docs/spec/platform-server.md:535`;
`docs/spec/engine.md:254`; `engine/run.go:163`, `:220`.

Verification for this explanation was source tracing, with an independent review of
publication ordering; no application code changed and no tests were run.
