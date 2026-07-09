# pnpm `corepack` smoke failure — root-caused to a poisoned Dagger cache; `platform clean` added

Resolves the "pnpm `corepack` smoke failure" item from
[`2026-07-09-resume.md`](2026-07-09-resume.md).

## Symptom

pnpm builds fail at `WithExec(["corepack","enable","pnpm"])` with
`exec: "corepack": executable file not found in $PATH`.

## Root cause (observed, not inferred)

A **poisoned Dagger v0.21 DagQL cache layer** for the n-install exec. The cached layer is
recorded as **success** (its captured stdout says `installed: v22.23.1`, exit 0) but its
filesystem snapshot is **missing everything `n` wrote to `/usr/local`** — node/corepack/npm
absent, `/usr/local/n` and `/usr/local/lib/node_modules` gone. Any build reusing that cache
key inherits an empty `/usr/local` and dies at `corepack enable`.

Ruled out along the way (each by direct observation, not reasoning):
- **Not the recipe.** A faithful standalone `docker run` of the identical steps installs
  corepack fine and `corepack enable pnpm` exits 0.
- **Not PATH.** wolfi's image-config PATH includes `/usr/local/bin`; the live in-container
  PATH after the real chain is `/platform/bin:/usr/local/sbin:/usr/local/bin:…`.
- **Not explicit platform.** A *fresh* n-install under the same explicit
  `Platform: "linux/arm64"` persists node+corepack and passes; only the *reused* poisoned
  layer fails. Explicit platform merely selects which cache key gets hit.

Isolation done with a throwaway dagger probe (`nprobe/`, see Cleanup) replaying
`BaseImageForUnit` + `withPNPMBase` and dumping the container fs.

## Trigger — unconfirmed

The Fable research agent built a runnable repro (~74 interrupt/restart cycles on
v0.21.7) and could **not** reproduce the silent poisoning. A plain timeout-kill only
yields a **safe** `commit output ref … context canceled` "cancel-bleed" (errors, not silent
success). Leading unproven suspects: the apk `--mount=type=cache` interacting with the layer
snapshot at commit, or a layer poisoned by an older engine build and still served. Dagger
v0.21.0 replaced the buildkit solver with DagQL (7 weeks before this incident); v0.21.4–.7
shipped four sibling cache-race fixes; **v0.21.7 is the latest — nothing to upgrade to.**

## Decision

**Upstream Dagger bug; no in-band code fix works.** Any integrity assertion appended to the
n-install exec (`… && test -x /usr/local/bin/corepack`) caches *alongside* the poison — the
`test` runs in the process's live view where the files exist, passes, exec exits 0, and the
truncation still happens after, at snapshot commit. A separate downstream check could only
make the build fail *loud* (not green) and still needs a cache prune to recover.

So the answer is operational, not a builder change:
- **`platform clean`** (new) prunes the local Dagger cache — first-line diagnostics for any
  "worked on a fresh checkout but not here" failure. `cmd/clean.go` → `engine.Clean` →
  `client.Engine().LocalCache().Prune(ctx)` across the discovered fleet.
- **No Dagger issue filed** (operator's call).
- **apk is NOT the fix.** Fable recommended `apk add nodejs-22 corepack`; rejected on the
  provisioning-taste reasoning now recorded in [`../../CLAUDE.md`](../../CLAUDE.md) (four
  uncoordinated maintainer groups; stay closest to the least-magic, most-reliable upstream).
  Do not re-propose it.

## Landed this session (uncommitted)

- `cmd/clean.go` + `engine.Clean` (+ `resolveHosts` shared with `Client`) + `main.go` wiring.
  Builds and vets green.
- `CLAUDE.md` build-facts: the Node/pnpm provisioning gist + `platform clean` as first-line
  diagnostics.
- Deleted the stale `2026-07-09-session-postmortem-instruction-fix.md` note.

## Cleanup still pending

- `rm -rf nprobe/` (throwaway dagger probe) and `docker rm nprobe` (exited sleep container).
- Fable's repro sits at `/tmp/dagger-repro` — throwaway, delete when done.

## Still open

- **pnpm smoke still red on a cold build.** `platform clean` recovers the poison, but a cold
  pnpm build tripped the 1-min module timeout (`context deadline exceeded`); cause
  unconfirmed — do not assert it's the tj/n download without measuring.
- Go build-speed regression — parked in
  [`2026-07-09-go-build-speed-regression.md`](2026-07-09-go-build-speed-regression.md).
- Builder-refactor WIP (Bucket B) still uncommitted, unrelated to this work — do not bundle.
- Logging slice `4ce8ee4` still unpushed.
