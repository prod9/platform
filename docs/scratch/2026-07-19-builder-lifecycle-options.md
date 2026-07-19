<!-- not spec/decision because: exploration; graduates to spec once the slice lands -->

# Builder lifecycle — structure

Shape of the **build-lifecycle slice**: `framework` emits a serializable `Plan` of typed
steps; a shared executor walks it firing a per-step observer; the srv runner becomes the
event-sourced reconciler fed by that observer. **This slice = Plan + srv + builder.** Hooks,
step-level cache, and a generic templating step are deferred phases (bottom).

## The settled frame — not re-opened

Direction is ruled (`.ace/save.ledger.md` item 9, 2026-07-18): the runner is an
**event-sourced reconciler**. No stored `state` column; an append-only **`BuildEvent`**
stream is the primitive; current-state / attempt / stuck / timed-out are folds of
`f(history, platform.toml timeout)`. What this doc adds is *where the events come from* — a
host-side observer on authored step boundaries.

Phases are **ours, not dagger's.** The platform binary owns orchestration; dagger runs what
we hand it. A `.Sync()` at a chosen point forces resolution and *is* the step boundary — no
dagger-execution-model pin to investigate.

## Core structure: framework emits a `Plan` of typed steps

Today the sixth `Framework` method is imperative — `Build(ctx, client, *BuildUnit)` execs
dagger ops inline and returns a synced container (`frameworks.md:24-28`); the steps are
implicit in Go control flow (`frameworks.md:136-143`). The rework **reifies the procedure
into data**:

1. **`Plan(*BuildUnit) Plan`** — pure, no client: emits an **ordered list of typed steps**.
   Typed → the UI tells a `Test` step from a `Compile` step; pure → hermetic-unit-testable
   and the thing you serialize for display.
2. **`Execute(ctx, client, Plan, observer) container`** — framework-side (raw client, no
   `engine` import), walks steps into dagger ops, `.Sync()` at each boundary, fires the
   observer per step.

Same artifact shown and run: the UI renders the `Plan`; `Execute` walks the same `Plan` — no
drift between shown and ran. Test-in-build becomes a **first-class visible `Test` step**
without loosening the gate.

**Every step that takes time is visible in the UI with its duration tracked — no
below-the-line steps, no coalescing.** Per-step timing is therefore a core requirement of the
observer/`BuildEvent`.

### `Plan` does not compete with `BuildAttempt` — orthogonal axes

- **`BuildAttempt` / `BuildUnit`** = *which* modules/images (one unit per module,
  `frameworks.md:178-183`). Unchanged; still the in-memory arg-bag.
- **`Plan`** = the ordered typed *steps* to build **one** unit (the *how*) — what
  `BuildUnit.command` collapses to a single opaque string today, made first-class.

`Build` is per unit (`frameworks.md:182-183`), so a `Plan` is **per-`BuildUnit`**. `Clone`
and `Publish` are **engine-level**, bracketing the per-unit work. Engine owns clone
mechanics, **parameterized**: srv supplies cache policy (mirror + per-build worktree,
`repoprep.go` today) and points engine at the target dir; local `publish` passes the existing
wd so clone is a no-op. Keeps "one publish engine, two drivers, identical path" honest — both
drivers hit the same engine clone step, differing only by argument — and thins srv to *cache
policy only*.

Serialization was never the crux: json tags on the step types, or a `build_attempt_view.go`
srv-side shim for API display. Not architectural.

## Step taxonomy — first stab (redlined 2026-07-19)

One **generic** menu. **No infra specialization** — Infra and Dockerfile flow through the
same steps as any app, filling a subset. `custom`/`gitops` special steps are gone: Infra's
render *is* its `Compile`; its pack *is* its `Package`; Dockerfile's build *is* its `Compile`.

**Engine-level** (brackets, per unit):

| Step      | Does                                    | Notes |
| --------- | --------------------------------------- | ----- |
| `Clone`   | acquire repo@ref → checkout dir         | srv → cached-mirror worktree; local → passes wd (no-op). Per build. |
| `Publish` | push built container @ resolved tag     | after `Execute` returns; `engine.Publish` today |

**Unit-level** (framework `Plan`; each framework fills a subset):

| Step        | Does                                                                      | Filled by |
| ----------- | ------------------------------------------------------------------------- | --------- |
| `Baseline`  | pinned Wolfi base + `apk upgrade` + build pkgs (`fw.md:112-134`)          | Go, pnpm  |
| `Toolchain` | pin/provision toolchain (Go `GOTOOLCHAIN`; Node via `n`+corepack)         | Go, pnpm  |
| `Deps`      | cache-keyed dep fetch (`go mod download`; `pnpm install`)                 | Go, pnpm  |
| `Test`      | run suite — the hard gate (`go test ./...`, `fw.md:136-143`)              | Go (pnpm TBD) |
| `Compile`   | produce the artifact — `go build` / `pnpm build` / static bundle / `host.DockerBuild` / `gitops.Render`→tree | all |
| `Package`   | assemble the shippable image — runner base + artifact, Caddy (static), or write tree into `FROM scratch` single-layer (infra) | all |

**Composition:**

- **Go** — Clone → Baseline → Toolchain → Deps → Test → Compile → Package → Publish
- **pnpm/static** — Clone → Baseline → Toolchain → Deps → Compile(bundle) → Package(Caddy) → Publish
- **Infra** — Clone → Compile(`gitops.Render`→tree) → Package(tree→`FROM scratch`, one layer) → Publish
- **Dockerfile** — Clone → Compile(`host.DockerBuild`) → Package(no-op; image already built) → Publish

Open: `Test` for pnpm — is there a suite today? (`frameworks.md` only specs the Go gate.)

## Observer surface (in scope) — the single extension point

`framework` exposes **one** per-step extension surface, fired at each `.Sync()` boundary.
`framework` defines the interface; consumers implement it, so `framework` keeps no hard dep
on any driver. Consumed by **both drivers, symmetrically** — CLI `publish` → per-step
progress to `buildlog`; srv → `BuildEvent`s. The surface allows both observing *and*
aborting; a consumer uses what it needs (the srv observer only observes).

## Observable timeline (display only)

- **UI = render the `Plan`, overlay the current attempt's per-step results.** The full
  timeline is engine composing `Clone` + `<framework Plan>` + `Publish`.
- **Persist each attempt's Steps as-run** so we can always display that attempt. That's the
  whole requirement — no back-compat, no cross-version reconciliation: the engine **always**
  uses the latest `Plan` from the framework; past attempts never feed execution, they are
  display records only.

## Multi-org: RULED OUT — one install per cluster

Decided (chakrit, 2026-07-19): **do not** pursue one-srv-many-orgs — cross-org would force
cross-cluster authentication, non-trivial and expensive. Model stays **one platform
installation per cluster** (one for prodigy9, fi, naxon, each client cluster).

- Install record stays a singleton; builds key on `owner/repo`. No `installation_id`
  insurance.
- Answers the original cross-org/cross-cluster question: delivery is **N installations, not
  a shared hook** — each cluster runs its own baseline Receiver + org webhook.
- Should graduate to a short decision note (one-install-per-cluster). Not writing it unprompted.

## Slice sequencing (this slice = Plan + srv + builder)

0. **Spec-update slice — precedes ALL implementation.** Graduate this design into `docs/spec/`
   — `frameworks.md` (the `Plan`/steps/`Execute`/observer contract change) and
   `platform-server.md` (the event-sourced reconciler + `BuildEvent`). Implementors get only
   the spec, so nothing here reaches them until it's spec'd. This is the real next step; the
   scratch doc is not a hand-off artifact.
1. **`Plan` type + `Plan(*BuildUnit)`** — typed-step decomposition; port one framework's
   imperative `Build` into pure emission. Settle the step taxonomy. Pure data → hermetic
   `go test`.
2. **Observer surface + `Execute(client, Plan, observer)` + `.Sync()` boundaries** — the
   shared step-walker, framework-side, firing the observer per step. Dagger-based → **smoke
   golden**, not the hermetic gate. `engine.BuildAndPublish` stays the driver above;
   `framework` never imports `engine`.
3. **`BuildEvent` schema + `build_events` table + pure fold** — `f(events, now, timeout)`;
   per-step results persisted (chain-invalidation model above); reconciler materializes a
   terminal `BuildTimedOut` when fold+clock crosses threshold (subsumes orphan recovery). Fold
   is hermetic; the runner is smoke.
4. **Reconciler rebuild** — srv registers the observer, appends `BuildEvent`s, cuts the
   `status` column over to projected-from-events (versioned per-step results, back-compat).
5. **Observability projection / API** — UI renders the `Plan` matched to the attempt's
   per-step results (display only).

## Deferred phases (not this slice)

- **Hooks** — every step becomes implicitly hookable via a mini-syntax (`before:deps`,
  `after:test`) on the observer surface; repo `.platform/*.sh` scripts attach there. **Abort
  IS a hook power** — a hook exiting nonzero aborts the build chain (distinct from the srv
  observer, which only reads). Runs in-engine so local and server behave identically.
- **Step-level cache** — reuse unchanged steps across rebuilds instead of re-running every
  step.
- **Generic `Render`/templating step** — a generic optional step any framework may emit or
  skip (chakrit's "use for templating later"); today infra's render just fills `Compile`.

## Blockers — RULED 2026-07-19

- **Item 13 — Slice 3b state-model → `reorder`.** State order: db → app-credentials →
  migrations → app-installed(=record-exists). Claim via the GitHub App Setup URL →
  `GET /api/install/claim`, org-owner-gated (resolve installation→org, verify owner, write
  record). Unblocks Slice 3b.
- **Item 14 — CMD-NOUN → `srv` canonical, `serve` aliases.** `platform srv` starts the
  server and is the noun-group hosting `srv data migrate`; `platform serve` is a back-compat
  alias.

## Constraints every part must satisfy (from the record)

- **Test-in-build hard gate** — `Plan` emission + the fold stay hermetic. **Anything
  dagger- or docker-based (`Execute`, `Clone`, real step runs) goes in the smoke golden
  tests (`./test.sh`), never the `go test` gate.**
- **One publish engine, two drivers** — do not touch `engine.BuildAndPublish`; the runner
  stays a driver. `cli` must not import `srv`; shared pkgs must not import server concerns;
  `framework` must not import `engine` or `srv` (observer iface lives framework-side).
- **Pull model** — no Flux→srv webhook; observability is srv reading Flux CR state via the
  pod ServiceAccount only.
- **Zero platform RBAC** — no permission tables; authz stays GitHub-delegated.
- **Split log channels** — build-side console via `internal/buildlog`; server via `fxlog`.
- **Boot discipline** — no auto-migrate at boot; boot only starts the worker; no boot-time
  requeue-orphans (the timeout fold subsumes it).
