<!-- not spec/decision because: unruled options for who owns the dagger session's lifetime;
one of these becomes a decision doc once chakrit picks -->

# Who owns the dagger session? — options

## The defect, stated once

Five commands share one shape (`cmd/build.go:26`, `export.go:25`, `exec.go:36`,
`preview.go:46`, `publish.go:62`):

```go
eng := engine.New(fxconfig.Configure())
defer eng.Close()
ctx := engine.NewContext(context.Background(), eng)
results, err := engine.Build(ctx, cfg, args, newObserver())
```

`defer eng.Close()` *looks* like the lifetime guard. It is not. Inside `build`
(`engine/engine.go:207-209`) each unit gets `context.WithTimeout(ctx, unit.Timeout)` and a
`defer cancel()` that fires when the unit's closure returns — before `Build` returns to its
caller. That per-unit ctx is what reaches `dagger.Connect` (`run.dial` → `Engine.Client` →
`clients.Get` → `dialEngine`, `clients.go:34`), and Dagger binds the **session** to its dial
context. So the session dies at the end of the unit, while the pooled client
(`clients.go:69`) and the `BuildResult.container` both live on.

`build` alone survives this because it never touches the container afterwards. The three
that do — `export.go:40`, `exec.go:52`, `preview.go:60`, all via `UnsafeDagger` — get
`file already closed`, surfaced as a Dagger 502. They are also precisely the three with zero
smoke coverage, which is why the suite stayed green.

**The `defer cancel()` is correct and must stay.** It is mandatory for `WithTimeout`
(lostcancel), and it enforces `unit.Timeout` — default 1m (`conf/conf.go:98`), spec'd at
`engine.md:99` as wrapping the whole run so a hung step fails that unit without aborting
siblings. The bug is not that the unit scope ends; it is that the *session* was scoped to it.

Rejected already: `context.WithoutCancel` at the dial (landed as `23d62a8`, to be reverted).
It works, but it makes a dial immune to *all* cancellation including real shutdown, and it
leaves session lifetime defined by whichever ctx happened to be laundered rather than by an
owner.

## The axis

Two questions, and each option answers both:

1. **Who owns the session** — the Engine (process), a build batch, or a single run?
2. **Does a container ever outlive the scope that built it?** Every option except D says yes
   and must therefore name a scope that outlives it.

## Option A — `Build` returns a closable batch

`engine.Build` stops cancelling on the caller's behalf and hands ownership out:

```go
builds, err := engine.Build(ctx, cfg, args, newObserver())
defer builds.Close()
for _, result := range builds.Results() { ... }
```

The batch owns one ctx that outlives every unit; each unit still derives its own
`unit.Timeout` child for its *steps*. `Close` cancels the batch, ending the session.

- **Fits the ask literally** — the thing whose lifetime matters becomes an object with a
  `Close()` that commands defer.
- **Makes the boundary visible** where it is currently invisible. A caller can see that
  holding a container past `Close` is wrong, because there is a `Close` to point at.
- Signature change at 5 call sites plus `BuildAndPublish` (`engine.go:121`), which would
  close immediately after `Publish`.
- Two closables per command (`eng.Close()` and `builds.Close()`) — an ordering rule
  (`builds` before `eng`) that nothing enforces. Ranking risk: mildly confusing surface.
- `build` returning `([]BuildResult, error)` today is what the multiplexer produces
  (`engine.go:215`); wrapping it is additive, not a rewrite.

## Option B — the Engine owns the session; the unit ctx bounds only steps

Keep `Build`'s signature. `Run.dial` stops using the step ctx: the Engine holds a base ctx
(from `New`, or `newClients`), the pool dials on that, and `unit.Timeout` is applied only to
the `Execute`/`Sync` calls inside `Run.execute` (`run.go:79-94`).

- **Smallest diff, zero call-site churn.** No new closable.
- **Makes `defer eng.Close()` truthful** — it becomes the only thing that ends a session,
  which is already what the spec claims (`engine.md:30`, `clients.go:14`: "nothing is ever
  closed mid-build", "`Close` only runs at shutdown"). The code would finally match.
- Engine stores a ctx on the struct — Go style dislikes it, though a pool with a background
  lifetime is exactly the case where `sql.DB` does the same in effect.
- **Does not lift anything a level.** The lifetime stays implicit; nothing in a command
  reads differently, so the next person can make the same mistake. It fixes the bug and not
  the legibility.
- Also incidentally fixes `cmd/ls.go:46` (the fourth `Engine.Client` caller, ledger 28's
  open item) for free, since it never enters a unit scope at all.

## Option C — `Run` gets an explicit `Close`

Push ownership down to the run rather than up to the batch: `NewRun` opens the scope,
`run.Close()` ends it, and `Result()` is only valid before `Close`. `build` closes each run
as it finishes; the three container commands drive a single run directly and defer `Close`
themselves.

- Ownership sits exactly where the container does — `Run` already owns both container and
  client (`run.go:32-33`), so the object that holds the resource is the object that frees it.
- Honest about the one-container-one-session coupling that `UnsafeDagger` documents
  (`engine.go:177-179`).
- But the three commands currently reach their container *through* `engine.Build`, which is
  a fan-out. Driving a bare `Run` means either exposing `NewRun` as the single-unit path (two
  public ways in) or `Build` handing back live runs instead of results — which un-does
  `Result()`'s guarantee that scalars can only come from the finished stream
  (`run.go:135-141`).
- Ranking risk: highest chance of dragging the observer/fold design back open.

## Option D — no container outlives its run; the engine grows the verbs

Give `engine` the verbs ledger 28 parked (`Exec`, `Export`, `Serve`/tunnel, `Inspect`) so
export/exec/preview express their work *inside* the run's scope. `UnsafeDagger` is deleted;
no container ever crosses a scope boundary, and the bug becomes unrepresentable.

- The only option that removes the failure mode rather than lengthening the lifetime.
- Matches the spec's own direction: `engine.go:174` already says these three reach through
  "until the engine grows verbs of its own for them; no new caller joins them."
- **Re-opens a settled call.** Ledger 28: chakrit weighed exactly these five verbs (options
  A/B/C) and chose the one-door hatch instead, so item 19's park holds. This says that park
  is what costs us the lifetime bug — presented as new evidence, not as a re-litigation.
- Largest change by far, and it lands interactive concerns (a shell, a tunnel) inside the
  engine, which is the thing the park was avoiding.

## Recommendation

**B now, D later, A if the ask is legibility.** B is the honest minimal fix: it makes the
code match a contract the spec already states, needs no signature churn, and clears the
`ls.go` item too. It leaves the legibility gap — nothing in a command shows where the
session ends — which is what A buys, at the price of a second closable and an unenforced
ordering. D is the real answer but it reverses a ruling; worth doing only when the engine
verbs are wanted for their own sake.

Independent of the pick: revert `23d62a8` and its spec commit first.

## Test that must survive any pick

`engine/clients_test.go` currently asserts the `WithoutCancel` mechanism, so it is
option-A/B/C-specific and dies with the revert. The behavior worth locking instead is
end-to-end and mechanism-free: **a container is still usable after the run that built it
returns** — build one unit through the public path, then touch the container, and expect no
error. That is the assertion that would have caught this, and none of `go test`'s current
coverage can host it because a hermetic stub cannot carry a container (a zero
`&dagger.Container{}` panics in `.Sync()`). So it belongs in the Dagger-path Go tests
already on the backlog, or as the first `export` case in `tests.cue`.
