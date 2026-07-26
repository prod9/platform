# A `*dagger.Client` is a session, not a poolable connection

**Date:** 2026-07-27
**Status:** accepted
**Spec:** [`../spec/engine.md`](../spec/engine.md) — §A `*dagger.Client` is a session,
§`Session` — the unit of lifetime

## The ruling

`engine/` stops pooling clients and grows a `Session` type: the span during which the
containers it produced are usable. The endpoint roster becomes stateless free functions
(`Hosts`, `Dial`); `Engine`, the client pool, the round-robin cursor, and
`engine.NewContext`/`FromContext` are deleted. A session dials as many connections as its
work needs, into its own context, and closes them together. `Publish` as a standalone verb
is deleted; publishing is a bracket inside the run.

## Why this is not the obvious default

The obvious default is what the code did: pool connections like `sql.DB`, which the package
stated as intent in two places. That analogy is wrong here, and the wrongness is invisible
until it costs you a day.

**A connection pool is an abstraction for a *fungible* resource.** Take one, use it, hand it
back; nothing you still hold depends on which one you got. A Dagger session is the opposite:
`dagger.Connect` opens a session and every container is a handle *into* it. Containers do
not survive it, cannot move between sessions, and cannot be re-acquired. Pooling a
non-fungible resource with a fungible-resource abstraction is the root error, and every
lifetime bug we hit descends from it.

## What it cost before we named it

`platform export` returned a Dagger 502 — `Post "http://dagger/query": file already closed`
— on a build that had just succeeded. The chain: `Build` gave each unit a
`context.WithTimeout(ctx, unit.Timeout)` and cancelled it when the unit finished; that ctx
reached `dagger.Connect` through the run's dial; Dagger binds a session to its dial context;
the client was cached process-wide, so the first unit's `cancel()` killed a session every
later caller still held. `build` never touches its container afterward, so the suite stayed
green — the three commands that do (`export`, `exec`, `preview`) are exactly the three with
no smoke coverage.

Two false starts worth recording, because both look reasonable:

- **`context.WithoutCancel` at the dial.** Works, and it is wrong: it makes a dial immune to
  *all* cancellation including shutdown, and it leaves session lifetime defined by whichever
  ctx happened to be laundered rather than by an owner. Landed as `23d62a8`, reverted.
- **Give the batch or the run a `Close()`.** Cannot work: the pool was keyed per *endpoint*
  and shared across units, so closing a per-build scope would tear down a session a sibling
  was still using. No per-build scope can own a shared pool.

Both attempts argued about *how long to prop the session up*. Neither asked *what it is* —
which is the question that dissolves it.

## Consequences taken deliberately

- **`BuildResult` carries no client.** The field existed for one line, `client.SetSecret`,
  and only because publish was a separate pass in time. As a bracket the connection is a
  local variable. This also deletes a latent bug: when `build.client` was nil the old path
  dialed a *different* connection and attached a secret from one session to a container from
  another.
- **`unit.Timeout` bounds steps, nothing else.** It no longer reaches a dial and does not
  limit how long a container stays usable.
- **Round-robin becomes uniform random choice at dial.** Same distribution, no state, so the
  roster stays a roster. This narrows the *mechanism* named in
  [2026-06-21 — Dagger engine StatefulSet + TCP](2026-06-21-dagger-engine-statefulset-tcp.md);
  that decision's topology (headless Service, pod-IP resolution, one engine per job via
  `dagger.WithRunnerHost`, job-granularity spreading) is unchanged and still accepted. Only
  "a round-robin cursor" becomes "a uniform random pick".
- **`ls` stops being a special case.** It reaches `Session.Unsafe()` like any ad-hoc caller,
  closing the open item recorded in the engine-opacity work.
- **Data may ride in the context; resources may not.** `cfg` travels via
  `config.NewContext`/`FromContext`; a `Session` is passed explicitly. Stated because the two
  are indistinguishable at a callsite, and carrying the engine on a context is what hid its
  lifetime in the first place.

## Left open

Whether a long-lived `srv` session needs liveness handling as engine pods come and go, or
whether `srv` opens a session per build and the question dissolves. The old pool re-pinged
and redialed; a command-length session never needs that. Decide before the srv build worker
lands.
