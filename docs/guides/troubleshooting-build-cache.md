# Troubleshooting the build cache

A build fails here that succeeds on a fresh checkout of the same commit. Nothing in
your tree changed the recipe, yet the container won't build. This is almost always a
**poisoned Dagger build cache**, not a bug in the builder. Fix it operationally.

## First line: `platform clean`

Run it before investigating anything else:

```
platform clean
```

It prunes the local Dagger build cache across the discovered runner fleet
(`LocalCache().Prune`). That evicts the poisoned layer; the next build re-derives it
from scratch. Reach for it as the **first-line diagnostic for any "worked on a fresh
checkout but not here" failure** — the fresh checkout worked because it never hit your
poisoned cache key.

`platform clean` is safe and cheap relative to the confusion of chasing a phantom recipe
bug. When a build failure smells like stale state, prune first, then re-run.

## The failure mode

Dagger v0.21 replaced its buildkit solver with DagQL. Under DagQL a cache layer can be
recorded as **success** — captured stdout intact, exit 0 — while its committed
filesystem snapshot is **missing files the exec actually wrote**. The layer's live
process view had the files; the snapshot taken at commit does not.

The canonical hit is the pnpm Node provisioning step. The `n` install exec caches as
`installed: v22.23.1`, exit 0, but its snapshot has an empty `/usr/local` — no
node, no corepack, no npm. Every later build reusing that cache key inherits the empty
tree and dies downstream at:

```
exec: "corepack": executable file not found in $PATH
```

The symptom points at PATH or the recipe; the cause is the snapshot. A standalone
`docker run` of the identical steps provisions Node fine — which is exactly why a fresh
checkout (fresh cache key) passes and your machine doesn't.

## Why there's no in-band fix

You cannot assert your way out inside the builder. An integrity check appended to the
install exec (`… && test -x /usr/local/bin/corepack`) runs in the process's **live**
view, where the files exist — it passes, the exec exits 0, and the truncation still
happens afterward at snapshot commit. The check caches *alongside* the poison. A
separate downstream check can only make the build fail **loud** instead of green, and it
still needs a cache prune to recover. So the fix is operational (`platform clean`), not a
builder change.

This is an upstream Dagger bug. v0.21.7 is the latest release — there is nothing to
upgrade to.

## Never switch pnpm → apk

When Node provisioning fails, `apk add nodejs corepack` is **not** the fix and must not
be proposed. The provisioning path (Node from nodejs.org via tj/n, pnpm via Node's own
corepack) is deliberate; see the Node/pnpm note in
[`../../CLAUDE.md`](../../CLAUDE.md). A cache or build failure is never a reason to change
where Node comes from. **Shed the cache with `platform clean`, then fix the real cause.**

## Known caveat: cold pnpm build

`platform clean` recovers from the poison, but a **cold** pnpm build (empty cache) has
also tripped the 1-minute per-module timeout with `context deadline exceeded`. The cause
is unconfirmed — do not assume it's the tj/n download without measuring. If a cold build
times out immediately after a prune, that's this open thread, not a fresh poisoning: warm
the cache and re-run before digging in.
