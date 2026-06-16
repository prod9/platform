# platformv2 — Implementation Plan

**Status:** confirmed (2026-06-16) · execution not started (Red on Slice 1 next) ·
supersedes the ad-hoc ordering in `PLANS.md`.
**Reads against:** `docs/spec/platform.md`, `config-allocation.md`, `gitops-build-plan.md`,
and `docs/decisions/*`.

## Framing

Spine-first, incremental monorepo — the spec anchor (`platform.md` § Anchors). A big-bang
`api/ cli/ ui/ core/` restructure as step one is the wrong move: it churns the test harness
(`test.sh`/`tests.cue`/testbeds), Dockerfile, and bootstrapper for zero functional gain.
Build along the spine (build → render → publish → reconcile); migrate the monorepo as new
code lands.

**Aggression (2026-06-16):** chakrit has no mission-critical workloads deployed, so we may
chunk slices and replace the live delivery path (Keel → Flux) freely. Calibrated into the
slice sizes below.

## Decisions — status

- **Spine-first** (Phase A delivery before Phase B server/RBAC) — *taken* (2026-06-16). The
  server *orchestrates* the spine, so build the orchestrated thing first; the spine has no
  identity deps (buildable now); the server is the biggest/riskiest piece. RBAC is the
  rewrite's *justification* but not its first build.
- **New code born in `core/`**, flat packages migrate later (opportunistic, not an up-front
  B1 restructure) — *taken* (2026-06-16). Slice 1 doesn't import `builder/`/`project/`/etc.,
  so an up-front move is pure churn that blocks first delivery behind a harness rewrite, on
  an unvalidated layout. New code goes straight to its final home; old flat packages move
  once in B1, after the spine has proved the layout.
- **Render runs in Dagger** (timoni-in-container), not on the host — *taken*. timoni is
  absent on host; hermetic per spec; host install is banned + unneeded.
- **Slice 1 = render + publish, merged** — *taken* (2026-06-16). Aggression: produce the
  consumable OCI config artifact end-to-end, not render-only.
- **Package name `core/gitops`** — *taken* (2026-06-16, adjustable). Names the delivery
  mechanism (pull-based GitOps via timoni+Flux+OCI), matching the spec framing.

## Phases

### Phase A — delivery spine (no server)

- **Slice 1 — render + publish.** `timoni build` an infra CUE module (in Dagger) → rendered
  manifests → push as the OCI config artifact under a **moving** per-env tag. Pure code +
  Dagger; locally testable; no cluster. Detailed below.
- **Slice 2 — reconcile + cutover.** Install Flux (source + kustomize controllers, OCI),
  `OCIRepository` on the moving tag, `prune: true`; inventory Keel workloads; migrate
  workload-by-workload; retire Keel (they fight over the image field otherwise). **Env
  prereq:** a reachable cluster + working `flux`/`kubectl` (host `kubectl` is broken — run
  from a cluster-admin context). Mostly manifests + ops; lands in `platform-init`.

### Phase B — control plane (the RBAC justification)

Prereq: **fx v0.4.0 → v0.8.2 bump** (PLANS.md #3) before any server code — pulls in fxlog
and the cmd-API drift. Chunk aggressively:

- **B1 — monorepo firm-up.** Move existing packages into `core/` (`builder`, `project`,
  `releases`, `gitctx`, types), split out `cli/`; one Go module across `api/cli/core`. Fix
  `test.sh`/`tests.cue`/testbeds/Dockerfile. Lower-risk now — spine code already proved the
  layout.
- **B2 — server skeleton + identity.** `api/` on fx + Postgres: health, migrations,
  `users`/`identities` schema, GitHub device-flow OAuth → platform token, `platform login`.
- **B3 — Projects + RBAC.** Project entity, members/roles, repo binding (`platform.toml`),
  audit. API + CLI.
- **B4 — gated deploy.** Authed user → write the immutable image ref into `infra/` CUE
  author-as-user (GitHub App) → triggers Slice 1's render+publish. Couples build+config so
  an unbuilt image can't be referenced.
- **B5 — brokers.** Kube token (`TokenRequest`, pod SA, exec-credential `kubeconfig`);
  secret-pull init-container.
- **B6 — UI.** SvelteKit (plain JS), adapter-static, `go:embed` into `api`. v1: Login,
  Projects, Access, Deploys, Target status (Flux CR status).

### Phase C — fold-ins (detail in `PLANS.md`)

infra-cli generators → `platform-init` baseline · **#7** version/provenance injection into
runner images · **#4** container hardening (non-root etc.) · **#5** plog → fxlog (rides B's
fx bump).

## Slice 1 — render + publish (ready to execute)

**Goal:** render an infra CUE module to manifests, then publish them as the OCI config
artifact. Start with render → stdout, then add publish — one slice, two commits.

**Code (born in `core/`):**

- `core/gitops/render.go` — run `timoni build` in a Dagger container (timoni binary on
  `wolfi-base`), image/env parameterized, capture the rendered manifests.
- `core/gitops/publish.go` — push the rendered manifests as an OCI artifact (moving per-env
  tag). Reuse the registry-creds pattern from `builder/` (`REGISTRY*` fx config); oras-style
  push via Dagger or an `oras` container.
- `cmd/render.go` (+ `cmd/publish.go`, or one combined command) — cobra wiring; stays in
  `cmd/`, no premature `cli/` split. Package name `core/gitops` is adjustable.

**Fixture:** `testbeds/infra-basic/` — a minimal timoni module (one Deployment + Service,
parameterized image tag), the thing we render. Authoring it = learning timoni's module
layout (`timoni.cue`, `templates/`, values) — the real work/risk of this slice.

**Test plan (TDD via the smoke harness):**

- **Red:** add a `render` case to `tests.cue` — `platform render testbeds/infra-basic
  --image x:y` → exit 0, output contains the Deployment with `image: x:y`. Fails (no cmd).
- **Green:** implement until it passes. Then repeat red→green for `publish` (push, then pull
  the artifact back and diff).
- **Broad:** `./test.sh` full suite stays green.
- **Caveat:** the timoni container is a new cold pull — if it brushes the 1m `tests.cue`
  timeout, warm the cache and re-run; do **not** raise the timeout.
- No unit test for the Dagger shell-out (needs the engine) — smoke is the right level.

## Environment & prerequisites

| Need              | State          | Action                                          |
|-------------------|----------------|-------------------------------------------------|
| timoni, oras      | absent on host | run in Dagger (host install banned + unneeded)  |
| dagger, cue, tofu | present        | ok                                              |
| kubectl, flux     | broken/absent  | Slice 2 only; run from a cluster-admin context  |
| fx                | v0.4.0         | bump to v0.8.2 before Phase B                   |
| cluster           | —              | Slice 2 prereq (Flux install + cutover)         |

## Open questions

- ~~`core/` migration: opportunistic vs explicit B1 restructure~~ — resolved: opportunistic.
- ~~Package naming: `core/gitops` vs `core/delivery`~~ — resolved: `core/gitops`.
- timoni `infra-basic` module structure — confirm against timoni docs while authoring.
- Two root trackers (`PLANS.md` + `TODOs.md`) — consolidate.
