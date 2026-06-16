# Config Allocation Map — platformv2

**Status:** living spec. Companion to [`platform.md`](platform.md) and the build brief
[`gitops-build-plan.md`](gitops-build-plan.md); frozen rulings in
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
  entities, identity/RBAC/access, audit, secret *values*, deploy history. Three clients of
  one API: **UI** (SvelteKit), **CLI**, **OpenTofu provider** — same backbone, no separate
  state.
- **`platform.toml`** (in each source repo, git) — per-repo build/project metadata + the
  infra pointer + which Project it binds to. Changes with the code.
- **`infra/` repo|path** (CUE, git) — per-target desired state: app image refs,
  components, replicas, env. The thing CI renders (`cue export`). No committed rendered
  YAML.
- **`tf/` repo|path** (OpenTofu, git) — the env/target list (and later cloud/DNS). Applied
  **manually, locally** in v2.
- **OCI registry** — app images (immutable tags) + the rendered config artifact (moving
  per-env tag). The source Flux pulls.
- **Flux** (in-cluster: source-controller + kustomize-controller) — reconciles the config
  artifact onto the cluster; applies/prunes; drift correction. No Argo. No Helm.
- **`platform-init` repo** (CUE, git) — the cluster baseline: Flux, cert-manager, NGF, the
  Dagger engine, platform itself. Seeded once (manual), then Flux-reconciled (except
  Flux's own lifecycle — never self-managed).

## Allocation table

| Config kind                           | Owner            | Form                            |
| ------------------------------------- | ---------------- | ------------------------------- |
| How to build a module                 | `platform.toml`  | TOML in source repo             |
| Project ↔ repo binding, infra pointer | `platform.toml`  | TOML in source repo             |
| Project entity, members, roles        | platform server  | Postgres (UI/CLI/tf-provider)   |
| Identity, linked accounts, audit      | platform server  | Postgres (`users`/`identities`) |
| Secret *values*                       | platform server  | Postgres, encrypted at rest     |
| Secret *references*                   | `infra/`         | CUE (init-container pulls)      |
| Per-target desired state (image, env) | `infra/`         | CUE (`cue export`) → OCI        |
| Env/target list                       | `tf/`            | OpenTofu (manual local apply)   |
| App image (the container)             | OCI registry     | immutable tag/digest            |
| Config artifact (Flux source)         | OCI registry     | moving per-env tag              |
| What's deployed where, drift          | Flux             | reconciles from OCI             |
| Cluster baseline (Flux/CM/NGF/engine) | `platform-init`  | CUE, Flux-reconciled            |
| Cloud / DNS                           | `tf/` (**v2.1**) | OpenTofu                        |

## Repos & artifacts

- **source repo** — app code + `platform.toml`. App CI (Dagger) builds the immutable
  image.
- **`infra/`** — per-target CUE. CI (= platform) runs `cue export` → multi-doc manifests →
  pushed as the OCI config artifact under a **moving** per-env tag. App image refs
  *inside* are **immutable**.
- **`tf/`** — OpenTofu env/target list; manual local apply in v2.
- **`platform-init`** — cluster baseline; one manual seed, then Flux.

## Deploy flow (where the surfaces meet)

1. App CI builds + pushes the immutable app image.
2. Gated deploy (CLI/UI, authed user) writes the image ref into `infra/` CUE,
   author-as-user via the GitHub App.
3. Platform (CI) renders `infra/` → pushes the config artifact to the moving env tag.
4. Flux's `OCIRepository` follows the tag → reconciles → pods run the pinned image.
5. Pods' init-containers pull their secrets from platform (outbound) at start.

No step pushes into the cluster. The gate is the CUE commit; everything after is pull.

## Phase boundaries

- **v2** — single home cluster; platform in-cluster; GitHub-only IdP; secrets via
  platform-pull init-container; `tf/` manual; no DNS.
- **v2.1** — DNS (Cloudflare via `tf/`), PR/branch deploys, the approvals/plan-gate UI,
  platform-run tofu.
- **phase 2** — multi-cluster (central control-plane + per-cluster agents); additional
  IdPs/service links (Google, Sentry, custom) via the `identities` table.
