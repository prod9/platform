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
  entities, identity/RBAC/access, audit, secret *values*, deploy history. Three clients of
  one API: **UI** (SvelteKit), **CLI**, **OpenTofu provider** — same backbone, no separate
  state.
- **`platform.toml`** (in each source repo, git) — per-repo build/project metadata + the
  infra pointer + which Project it binds to. Changes with the code.
- **`infra/` repo|path** (CUE, git) — the cluster's desired state: app image refs (committed
  literals the operator hand-edits — platform never rewrites this CUE), components, replicas, and
  per-env instances separated by k8s namespace. The thing CI renders (via the linked CUE engine,
  no `cue` binary). No committed rendered YAML.
- **`tf/` repo|path** (OpenTofu, git) — cloud/DNS provisioning (v2.1); not a platform env list.
  Applied **manually, locally**.
- **OCI registry** — app images (digest-pinned to dodge stale cache) + the rendered config
  artifact (moving tag; the infra repo's git history is the record). The source Flux pulls.
- **Flux** (in-cluster: source-controller + kustomize-controller) — reconciles the config
  artifact onto the cluster; applies/prunes; drift correction. No Argo. No Helm.
- **`platform-init`** (embedded in the tool) — the cluster baseline: Flux, cert-manager,
  NGF, the Dagger engine, platform itself. **Embedded** in platform (not a separate repo)
  as a **flat list** of `.cue` apps + `.platform` directives, **destination-encoded by name**;
  `ops init` installs each operator-chosen file to the destination its name encodes — the repo
  root, `apps/` (render-able components), or the mandatory `defaults/` package (shared defs like
  `#Basics`, imported by `apps/`). Seeded once (manual), then Flux-reconciled (except Flux's own
  lifecycle — never self-managed). See the
  [appliance ADR](../decisions/2026-06-17-opinionated-appliance-embedded-init.md) and the
  [flat-baseline ADR](../decisions/2026-06-22-flat-baseline-install-time-selection.md).

## Allocation table

| Config kind                           | Owner            | Form                            |
| ------------------------------------- | ---------------- | ------------------------------- |
| How to build a module                 | `platform.toml`  | TOML in source repo             |
| Project ↔ repo binding, infra pointer | `platform.toml`  | TOML in source repo             |
| Project entity, members, roles        | platform server  | Postgres (UI/CLI/tf-provider)   |
| Identity, linked accounts, audit      | platform server  | Postgres (`users`/`identities`) |
| Secret *values*                       | platform server  | Postgres, encrypted at rest     |
| Secret *references*                   | `infra/`         | CUE (init-container pulls)      |
| Desired state (image, replicas, env)  | `infra/`         | CUE (linked engine) → OCI       |
| App image (the container)             | OCI registry     | digest-pinned tag               |
| Config artifact (Flux source)         | OCI registry     | moving tag (git = record)       |
| What's deployed where, drift          | Flux             | reconciles from OCI             |
| Cluster baseline (Flux/CM/NGF/engine) | embedded         | files → `apps/`,`defaults/`,root |
| Cloud / DNS                           | `tf/` (**v2.1**) | OpenTofu                        |

## Repos & artifacts

- **source repo** — app code + `platform.toml`. App CI (Dagger) builds the immutable
  image.
- **`infra/`** — the desired-state CUE. CI (= platform) renders via the linked CUE engine → the
  `k8s/` manifest tree → pushed as the OCI config artifact under a **moving** tag (git history is
  the record). App image refs *inside* are committed literals, digest-pinned to dodge stale cache.
- **`tf/`** — OpenTofu cloud/DNS provisioning (v2.1); manual local apply. Not a platform env list.
- **`platform-init`** — cluster baseline, **embedded in the tool** as a flat, destination-encoded
  file list; `ops init` installs each chosen file to the destination its name encodes (repo root,
  `apps/`, or the mandatory `defaults/` package); one manual seed, then Flux.

## Deploy flow (where the surfaces meet)

1. App CI builds + pushes the app image (digest-pinned).
2. The operator **hand-edits** the app-image ref in `infra/` CUE and commits — platform never
   rewrites the CUE. The gate is GitHub push permissions on the infra repo (later, the server may
   author the commit as the user via the GitHub App).
3. `ops render` + `ops publish` push the config artifact to the moving tag.
4. Flux's `OCIRepository` follows the tag → reconciles → pods run the pinned image.
5. Pods' init-containers pull their secrets from platform (outbound) at start.

No step pushes into the cluster. The gate is the git commit; everything after is pull.

## Phase boundaries

- **v2** — single home cluster; platform in-cluster; GitHub-only IdP; secrets via
  platform-pull init-container; `tf/` manual; no DNS.
- **v2.1** — DNS (Cloudflare via `tf/`), PR/branch preview instances (infra CUE + namespacing),
  platform-run tofu. Gating stays GitHub push permissions — no separate approvals/plan-gate UI.
- **phase 2** — multi-cluster (central control-plane + per-cluster agents); additional
  IdPs/service links (Google, Sentry, custom) via the `identities` table.
