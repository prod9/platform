# platformv2 — Implementation Plan

**Status:** confirmed (2026-06-16) · **Slice 1 landed** (render `615caa4`, publish
`c9ffc0c`); Slice 2 (reconcile + cutover) next · supersedes the ad-hoc ordering in
`PLANS.md`. **Reads against:** `docs/spec/platform.md`, `config-allocation.md`,
`gitops-build-plan.md`, and `docs/decisions/*`.

## Framing

Spine-first, incremental monorepo — the spec anchor (`platform.md` § Anchors). A big-bang
`api/ cli/ ui/ core/` restructure as step one is the wrong move: it churns the test
harness (`test.sh`/`tests.cue`/testbeds), Dockerfile, and bootstrapper for zero functional
gain. Build along the spine (build → render → publish → reconcile); migrate the monorepo
as new code lands.

**Aggression (2026-06-16):** chakrit has no mission-critical workloads deployed, so we may
chunk slices and replace the live delivery path (Keel → Flux) freely. Calibrated into the
slice sizes below.

## Decisions — status

- **Spine-first** (Phase A delivery before Phase B server/RBAC) — *taken* (2026-06-16).
  The server *orchestrates* the spine, so build the orchestrated thing first; the spine
  has no identity deps (buildable now); the server is the biggest/riskiest piece. RBAC is
  the rewrite's *justification* but not its first build.
- **New code born in `core/` **, flat packages migrate later (opportunistic, not an
  up-front B1 restructure) — *taken* (2026-06-16). Slice 1 doesn't import `builder/`
  /`project/`/etc., so an up-front move is pure churn that blocks first delivery behind a
  harness rewrite, on an unvalidated layout. New code goes straight to its final home; old
  flat packages move once in B1, after the spine has proved the layout.
- **Renderer = `cue export`, not timoni** — *taken* (2026-06-16). See the
  [renderer ADR](../decisions/2026-06-16-renderer-cue-export-not-timoni.md). `cue` is on
  the host; infra-defs is the packaging layer; foreign manifests are patched by the
  [manifest patch DSL](../spec/manifest-patch-dsl.md). No timoni, no vendored k8s schemas,
  no Dagger forced for render.
- **Slice 1 = render + publish, merged** — *taken* (2026-06-16). Aggression: produce the
  consumable OCI config artifact end-to-end, not render-only.
- **Package name `core/gitops` ** — *taken* (2026-06-16, adjustable). Names the delivery
  mechanism (pull-based GitOps via `cue export` + Flux + OCI), matching the spec framing.
- **CLI namespace `ops` ** — *taken* (2026-06-17). The delivery spine is grouped under
  `platform ops` (`ops render`, `ops publish`); `render` moved there from top-level.
  Avoids colliding with the existing container-release `publish`. Full rationale and
  parked follow-ups in [slice-1 open questions](2026-06-17-slice1-open-questions.md).

## Phases

### Phase A — delivery spine (no server)

Order: Slice 1 (done) → the patch-DSL slices D1–D3 (Phase A′) → Slice 2. D1–D3 come
before Slice 2 because Slice 2 installs the baseline, and the baseline is the DSL's first
consumer.

- **Slice 1 — render + publish.** ✅ **Landed 2026-06-17.** `cue export` an infra CUE
  module → multi-doc manifests → push as the OCI config artifact under a **moving**
  per-env tag. Pure code; locally testable; no cluster. Detailed below.
- **Slice 2 — reconcile + cutover.** Install Flux (source + kustomize controllers, OCI),
  `OCIRepository` on the moving tag, `prune: true`; inventory Keel/argocd workloads;
  migrate workload-by-workload; retire Keel (they fight over the image field otherwise).
  **Depends on:** D1–D3 (the baseline install). **Env prereq:** a reachable cluster +
  working `flux`/`kubectl` (host `kubectl` is broken — run from a cluster-admin context).
  Mostly manifests + ops; seeded via the embedded baseline.

### Phase A′ — patch DSL + embedded init (the appliance baseline)

Pulled forward from Phase C (2026-06-17). The [manifest patch DSL](../spec/manifest-patch-dsl.md)
is the primitive for adapting foreign installs; the embedded baseline (the
[appliance ADR](../decisions/2026-06-17-opinionated-appliance-embedded-init.md)) is its
first consumer; bootstrap writes it into the infra repo. Port source:
`infra-cli/pipelines/*` + `pipelines/yamleditor`. Dogfood target: the real `infra` repo.

- **Slice D1 — DSL core (hermetic).** Port `yamleditor` path-walk (`Get`/`Set` over
  `map[string]any`/`[]any`, incl. the new `[name=v]` field-select form) + the in-buffer
  verbs (`select`, `set`, `set-if-absent`, `append`, `append-unique`, `delete`,
  `delete-doc`) + the directive parser. No network. Unit-tested on inline multi-doc
  fixtures. Born in `core/` (proposed `core/patch`). Detailed below.
- **Slice D2 — I/O verbs + emit.** `download URL` (port `pipelines/download.go`),
  `extract-zip`, `${var}` substitution, and `emit` → hand the buffer to Slice 1's publish
  pipeline. `${var}` values come from `platform.toml`'s generic `[ops.vars]`
  (`project.Ops.Vars map[string]string` — see the
  [generic-ops-vars ADR](../decisions/2026-06-17-generic-ops-vars-single-config.md)); no
  typed DTO. Network verbs fixtured/cached for tests, real fetch at runtime; checksum
  guard is an open question (below).
- **Slice D3 — init DSL package + bootstrap-writes-DSL.** Add `Ops.Vars map[string]string`
  (generic, no per-software fields); the per-component assembly layer reads it → fills the
  `${var}` map + gates directive lines on string-valued bools (`vars["nginx_experimental"]
  == "true"`). Author the embedded baseline (Flux seed + cert-manager + NGF + engine) as
  directive files; `go:embed` them; bootstrap writes them into the infra repo
  (write-once-then-owned, like `bootstrapper/`). Baseline bits we author (namespaces,
  RBAC, Gateway, platform Deployment) stay CUE; foreign ones are DSL. **Migration:** fold
  the
  infra repo's `settings.toml` into `platform.toml` (versions/flags → `[ops.vars]`;
  `maintainers`/`repo.url` → existing `maintainer`/`repository`) and delete it. Dogfood:
  reproduce `infra`'s `k8s/{cert-manager,nginx-gateway}` via directives.

### Phase B — control plane (the RBAC justification)

Prereq: **fx v0.4.0 → v0.8.2 bump** (PLANS.md #3) before any server code — pulls in fxlog
and the cmd-API drift. Chunk aggressively:

- **B1 — monorepo firm-up.** Move existing packages into `core/` (`builder`, `project`,
  `releases`, `gitctx`, types), split out `cli/`; one Go module across `api/cli/core`. Fix
  `test.sh` /`tests.cue`/testbeds/Dockerfile. Lower-risk now — spine code already proved
  the layout.
- **B2 — server skeleton + identity.** `api/` on fx + Postgres: health, migrations,
  `users` /`identities` schema, GitHub device-flow OAuth → platform token,
  `platform login`.
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

The patch DSL + init baseline moved to **Phase A′** (2026-06-17). Remaining: **#7**
version/provenance injection into runner images · **#4** container hardening (non-root
etc.) · **#5** plog → fxlog (rides B's fx bump) · residual infra-cli generators not
covered by the DSL port.

## Slice 1 — render + publish (landed 2026-06-17)

**Goal:** render an infra CUE module to manifests via `cue export`, then publish them as
the OCI config artifact. Landed as two commits — render → stdout (`615caa4`), then publish
(`c9ffc0c`).

**Code (born in `core/`), as landed:**

- `core/gitops/render.go` — runs `cue export -e objects --out yaml` over the module (cue
  is on the host), then walks the YAML sequence and emits each object as one multi-doc
  (`---`) document. Image injected via `--inject image=` into the module's `@tag(image)`.
  No Dagger.
- `core/gitops/publish.go` — packages the manifest stream as a single gzipped-tar layer
  and packs it with **Flux media types** (`…flux.config.v1+json`,
  `…flux.content.v1.tar+gzip`) via **oras-go**, pushed to any `oras.Target` under the
  moving per-env tag.
- `core/gitops/registry.go` — resolves `oci://host/repo:tag` and authenticates from
  `REGISTRY_USERNAME`/`REGISTRY_PASSWORD` (registry host comes from the ref; defined
  locally, not imported from `builder/`, to keep the spine decoupled).
- `cmd/ops.go` (parent), `cmd/ops_render.go`, `cmd/ops_publish.go` — cobra wiring under
  the `platform ops` namespace; stays in `cmd/`, no premature `cli/` split. The publish
  target is **convention-over-configuration** (2026-06-17): no `--to` flag — it comes from
  the `[ops]` section of `platform.toml`, where `image` is inferred from `repository`
  (`ghcr.io/x`) and `tag` defaults to `latest` (`project.Ops.Ref`). `--tag` overrides the
  moving tag for a per-env publish.

**Fixture:** `testbeds/infra-basic/` — a thin CUE module depending on `prodigy9.co/defs`
(infra-defs), declaring one app (Deployment + Service via a pack or wrappers) with a
parameterized image tag, exposing an `objects` list. The real work/risk is the
`cue export → multi-doc emit` shape and the infra-defs `CUE_REGISTRY` wiring, **not**
vendoring schemas — there are none to vendor.

**Test plan (TDD), as landed:**

- **Render:** smoke case in `tests.cue` —
  `./testbed.sh infra-basic ops render --image x:y` → exit 0, stdout snapshot contains the
  Deployment with `image: x:y`.
- **Publish:** Go unit test (`core/gitops/publish_test.go`) round-trips the manifests
  through a filesystem `oci.Store` — pushes, pulls every layer back, and asserts byte
  identity plus Flux media types. **No publish smoke:** a live-registry round-trip needs
  creds + network, which the 1m honest-timeout harness forbids; live push is validated
  manually / in Slice 2.
- **Broad:** `./test.sh` full suite stays green; `go test ./...` covers the unit side.
- **Caveat:** the fixture resolves `prodigy9.co/defs` from `ghcr.io/prod9` on first run —
  if the module fetch brushes the 1m `tests.cue` timeout, warm the CUE module cache and
  re-run; do **not** raise the timeout.

The manifest patch DSL ([spec](../spec/manifest-patch-dsl.md)) is **not** in this slice —
it adapts third-party installs and lands next, in **Phase A′** (Slices D1–D3). Slice 1 is
author-our-own-manifests only.

## Slice D1 — DSL core (ready to execute, next)

**Goal:** a hermetic, in-memory manifest patch engine — parse a directive file, apply the
buffer-editing verbs to a multi-doc YAML stream, no network. This is the bulk of the DSL
and the part that is cleanly unit-testable.

**Port source (read, don't import — separate repo):** `infra-cli/pipelines/yamleditor/`
(`yamleditor.go` 129 LOC, `_test.go` 138 LOC — path-walk `Get`/`Set` with int-index and
create-if-absent) and the verb shapes in `infra-cli/pipelines/edit_yaml.go`. The pipeline
ops there are imperative structs; D1 keeps the path-walk backend and replaces the
front-end with the directive parser.

**Code (born in `core/`, proposed `core/patch`):**

- `core/patch/yamledit.go` — port of `yamleditor`: `Get`/`Set` over `map[string]any` /
  `[]any`, plus the **new** field-select form `[name=v]` (the load-bearing path form —
  upstream reorders lists between versions, so int-index targeting is a latent bug).
- `core/patch/verbs.go` — the in-buffer verbs over a multi-doc stream: `select` (scope by
  `kind=`/`name=`/…), `set`, `set-if-absent`, `append`, `append-unique`, `delete`,
  `delete-doc`.
- `core/patch/parse.go` — the directive parser: one verb per line, `${var}` substitution
  (substitution only, no expressions), comments/blank-line handling.
- I/O verbs (`download`, `extract-zip`, `emit`) are **D2**, not here.

**Test plan (TDD, hermetic):**

- **Red→Green** per verb against inline multi-doc fixtures: assert the path-walk + each
  verb mutate the buffer correctly, incl. `[name=v]` selection, idempotency of
  `append-unique`/`set-if-absent`, and `delete-doc` dropping scoped docs. Pure Go, no
  network — runs under `go test ./core/patch/`.
- Reuse the existing `yamleditor_test.go` cases as a baseline for the ported path-walk.
- No smoke case (no CLI surface yet; the DSL gets wired to a command in D2/D3).

**Acceptance:** a directive file like the cert-manager example in the
[DSL spec](../spec/manifest-patch-dsl.md) parses and applies end-to-end in memory, verified
by unit assertions on the resulting stream.

## Environment & prerequisites

| Need              | State          | Action                                           |
| ----------------- | -------------- | ------------------------------------------------ |
| cue, dagger, tofu | present        | ok — render uses host `cue`, no timoni           |
| oras-go           | in go.mod (v2) | publish via the oras-go lib; no host `oras`      |
| kubectl, flux     | broken/absent  | Slice 2 only; run from a cluster-admin context   |
| fx                | v0.4.0         | bump to v0.8.2 before Phase B                    |
| cluster           | —              | Slice 2 prereq (Flux install + cutover)          |

## Open questions

- ~~`core/` migration: opportunistic vs explicit B1 restructure~~ — resolved:
  opportunistic.
- ~~Package naming: `core/gitops` vs `core/delivery` ~~ — resolved: `core/gitops`.
- ~~Renderer: timoni vs `cue export` ~~ — resolved: `cue export` (see renderer ADR).
- ~~`cue export` multi-doc emit shape~~ — resolved: top-level `objects` list, exported
  with `-e objects --out yaml` and split per element (`testbeds/infra-basic/infra.cue`,
  `core/gitops/render.go`).
- Two root trackers (`PLANS.md` + `TODOs.md`) — consolidate.
