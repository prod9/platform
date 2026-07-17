# Config Allocation Map — platformv2

**Status:** living spec. Companion to [`platform.md`](platform.md) and
[`architecture.md`](architecture.md); frozen rulings in
[`../decisions/`](../decisions/). Distilled from the 2026-06 design walk.

The job of this document: **exactly one owner per config kind.** Nothing settable in two
places, no gaps. The Flux pivot left us with several surfaces — this map is the discipline
that stops them overlapping.

## The invariant

**No cluster credentials exist outside the cluster.** Everything is pull-based: the
cluster reaches out (Flux pulls OCI artifacts; pods pull their secrets); nothing reaches
in. Platform runs *inside* the (single, v2) home cluster and acts through its pod
ServiceAccount — so even platform holds no external cluster credential. Every allocation
below is chosen to preserve this.

## Surfaces, as layers

Top (closest to the human) to bottom (closest to the metal). One owner each.

- **platform server** (in-cluster, pod SA, Postgres) — the control plane. Owns Project
  entities, identity, audit, secret *values*, delivery history. It does **not** own
  authorization — that is GitHub's (zero platform RBAC). Three clients of one API: **UI**
  (SvelteKit), **CLI**, **OpenTofu provider** — same backbone, no separate state.
- **`platform.toml`** (in each source repo, git) — per-repo build/project metadata + the
  infra pointer + which Project it binds to. Changes with the code.
- **`infra/` repo|path** (CUE, git) — the cluster's desired state: app image refs (committed
  literals the operator hand-edits — platform never rewrites this CUE), components, replicas, and
  per-env instances separated by k8s namespace. The thing CI renders (via the linked CUE
  evaluator, no `cue` binary). No committed rendered YAML.
- **`tf/` repo|path** (OpenTofu, git) — cloud/DNS provisioning (v2.1); not a platform env list.
  Applied **manually, locally**.
- **OCI registry** — app images (digest-pinned to dodge stale cache) + the rendered config
  artifact (moving tag; the infra repo's git history is the record). The source Flux pulls.
- **Flux** (in-cluster: source-controller + kustomize-controller) — reconciles the config
  artifact onto the cluster; applies/prunes; drift correction. No Argo. No Helm.
- **`platform-init`** (embedded in the tool) — the cluster baseline: Flux, cert-manager,
  NGF, the Dagger engine, platform itself. **Owned by the `Infra` framework**, which installs
  it unconditionally; seeded once (manual), then Flux-reconciled — except Flux's own
  lifecycle, never self-managed. The file set and its destination-encoding rules are
  canonical in [`scaffolding.md`](scaffolding.md#destination-encoded-files).

## Allocation table

| Config kind                           | Owner            | Form                            |
| ------------------------------------- | ---------------- | ------------------------------- |
| How to build a module                 | `platform.toml`  | TOML in source repo             |
| Project ↔ repo binding, infra pointer | `platform.toml`  | TOML in source repo             |
| Project entity + repo membership      | platform server  | Postgres (UI/CLI/tf-provider)   |
| Authorization (who may deploy)        | GitHub           | infra-repo push permission      |
| Identity, linked accounts, audit      | platform server  | Postgres (`users`/`identities`) |
| Secret *values*                       | platform server  | Postgres, encrypted at rest     |
| Secret *references*                   | `infra/`         | CUE (init-container pulls)      |
| Desired state (image, replicas, env)  | `infra/`         | CUE (linked evaluator) → OCI    |
| App image (the container)             | OCI registry     | digest-pinned tag               |
| Config artifact (Flux source)         | OCI registry     | moving tag (git = record)       |
| What's deployed where, drift          | Flux             | reconciles from OCI             |
| Cluster baseline (Flux/CM/NGF/engine) | `Infra` framework | files → `apps/`,`defaults/`,root |
| Cloud / DNS                           | `tf/` (**v2.1**) | OpenTofu                        |

## Repos & artifacts

- **source repo** — app code + `platform.toml`. App CI (Dagger) builds the immutable
  image.
- **`infra/`** — the desired-state CUE. CI (= platform) renders via the linked CUE evaluator → the
  `k8s/` manifest tree → pushed as the OCI config artifact under a **moving** tag (git history is
  the record). App image refs *inside* are committed literals, digest-pinned to dodge stale cache.
- **`tf/`** — OpenTofu cloud/DNS provisioning (v2.1); manual local apply. Not a platform env list.
- **`platform-init`** — not a repo: the cluster baseline is embedded in the tool and owned by
  the `Infra` framework ([`scaffolding.md`](scaffolding.md#destination-encoded-files)); one
  manual seed, then Flux.

## Delivery flow (where the surfaces meet)

1. App CI builds + pushes the app image (digest-pinned).
2. The operator **hand-edits** the app-image ref in `infra/` CUE and commits — platform never
   rewrites the CUE. The gate is GitHub push permissions on the infra repo (later, the server may
   author the commit as the user via the GitHub App).
3. `publish` (infra is a framework) builds the manifest tree into a `FROM scratch` image and
   pushes it to the moving tag — the ordinary publish path, no bespoke OCI pusher. See
   [infra-publishes-as-plain-image-retire-oras](../decisions/2026-07-05-infra-publishes-as-plain-image-retire-oras.md).
4. Flux's `OCIRepository` follows the tag, extracts the layer via `layerSelector` →
   reconciles → pods run the pinned image. The publish fires GitHub's `registry_package`
   webhook → the GitHub→Flux `Receiver` (one per cluster, `name: "*"` — pokes every
   `OCIRepository` in every namespace) triggers a near-instant reconcile; the
   `OCIRepository` poll interval is only the fallback when the Receiver misses a delivery.
5. Pods' init-containers pull their secrets from platform (outbound) at start.

No step pushes into the cluster. The gate is the git commit; everything after is pull.

## Phase boundaries

Canonical in [`platform.md`](platform.md#phase-boundaries) — this doc scopes config
ownership, not phasing. The `tf/` row above is the only allocation a phase boundary moves
(v2.1).
