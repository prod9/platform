# Infra conversion + platform delivery migration plan (DRAFT)

**Status: DRAFT — pending chakrit's review.** Open decisions at the end are not settled.
Supersedes the partial 5-step sketch in the platformv2 implementation-plan breadcrumb
(whose "rewrite to house idiom" step is voided — see framing below).

## Goal

Move `../infra` (`prod9/infra`, stage9-targeting) off ArgoCD+Keel onto `platform ops
render` delivery, and stand platform up on the new committed-literal-image model. Flux
reconcile is a later slice; until then the rendered tree is applied directly (gated).

## Ground truth (infra agent, 2026-06-26)

- ArgoCD + Keel **REMOVED** from stage9 (`argocd`/`keel` ns deleted). All 16 Apps orphaned
  → fleet apps still **running but un-reconciled** (no selfheal/prune, no image auto-update).
- `../infra` teardown **committed** (`a88792e`, `96dbe2b`); infra-agent WIP committed.
  Working tree now holds only platform.* leftovers: `apps/platform.cue` (M, baseline form),
  `k8s/platform/` (D, old render), `platform.toml` (??).
- Live `deploy/platform` 2/2 = **new model already**: `image:latest`,
  `imagePullSecrets=[ghcr.io-pull-secret]`, `use-dagger-engine` grant. `dagger-engine` STS
  2/2. Serving `platform.prodigy9.co` (200).
- `defs` pinned **v0.3.19**. v0.4.0 published but **breaks n8n/x9** (`#Postgres`/`#Redis`
  auto-emit default-deny NPs) — **do NOT bump** in this migration.

## Corrected framing (drives every phase)

- `packs.#WebApp` is the **infra-defs everywhere-idiom**, not a repo-local style.
- `defaults` is **promoted** from `../infra`-local sugar to a **baseline-shipped,
  ops-init-written package** present in every platform-managed repo. That reconciles "don't
  strong-arm platform.cue into a local idiom" with "platform.cue should use defaults":
  defaults becomes part of the platform appliance.
- Image ref = **committed CUE literal**, never an ops var (committed-image ADR,
  2026-06-26). Bumping the version = committing a new literal.
- Pull secret is **required** — stage9 has no global kubelet creds (verified: a pod without
  `imagePullSecrets` → `ErrImagePull`). `defaults.#Basics` provisions the dockerconfigjson
  from a registry password; apps derive `#pull_secret: nsp.#out.pull_secret_name`.

## Phase 0 — platform image: real release + publish (platform repo, cli mode) · MINE

`platform.toml` strategy = `semver`; latest tag `v0.8.2`. Replace the `:latest` placeholder
with a real immutable tag.

1. `platform release` → cut a real version tag.
2. `platform publish` → build + push `ghcr.io/prod9/platform:<tag>` (needs `REGISTRY_*`
   creds; image is private).
3. Pin `<tag>` as a committed CUE literal in the baseline `platform.cue` (Phase 1).

## Phase 1 — defaults as a baseline package + baseline rework (platform repo) · MINE

1. **New baseline files** (embedded in `core/baseline`, written by `ops init` into
   `<repo>/defaults/`):
   - `defaults/basics.cue` — `#Basics` over `packs.#Basics`; ship
     `#registry_username: "CHANGEME"` / `#registry_password: "CHANGEME"` placeholders.
   - `defaults/gateway.cue` — `#gateway_name` / `#gateway_ns`.
2. **Module path = init input, always (supplied-as-facts — wrinkle #1):** `ops init` always
   takes the module path (prompt / arg / `ALWAYS_YES` default), **not** greenfield-only, and
   uses it as the authoritative fact for BOTH the `cue.mod` scaffold and templating the
   defaults import. No read-back from an existing `cue.mod` — avoids a CUE-parse step and the
   "when do I prompt?" branch. `cue.mod` stays **write-once** (`HasCueModule` bool — scaffold
   iff absent; no `LoadCueModule` needed). Operator running init on an existing-module repo
   supplies a matching path or accepts a broken import (knowingly); the common first-time path
   shares the one supplied value across `cue.mod` + `defaults/` + apps.
3. **Template the defaults import** in baseline `.cue` (`import "<module>/defaults"`) with
   the resolved path. **Amends the flat-baseline ADR** (2026-06-22): these `.cue` files join
   the templated set (where `platform.toml` + the `platform` script already live).
4. **Rework baseline `platform.cue`**: `defaults.#Basics` + `defaults.#gateway_*` + derived
   `#pull_secret` + committed-literal image (Phase 0 tag). Keep the engine as `defs.#StatefulSet`
   (no engine pack exists).
5. **hello-world app** (new baseline file): uses `defaults.#Basics` +
   `nsp.#out.pull_secret_name` with a **public image** (tiny http echo) so a fresh init
   deploys and demonstrates/pre-wires the pull-secret pattern without needing the operator's
   token first.
6. **Write-once defaults**: `ops init` must not overwrite an existing `defaults/` (operator's
   `CHANGEME`-filled token). Same preserve discipline as `mergeOpsVars`.
7. Update tests + smoke goldens.

## Phase 2 — `../infra` cutover · MINE (repo) / infra agent (cluster, gated on chakrit)

1. `ALWAYS_YES=1 platform ops init` into `../infra` (NOT `--force`). `../infra` already has a
   customized `defaults/` (real token, `listener_set.cue`) → **write-once leaves it intact**;
   only `platform.cue` (real pinned tag) + `platform.toml` (+ hello-world?) land. `defs`
   stays v0.3.19.
2. `platform ops render ../infra`. **Render scope is an open decision** (below) — renders
   ALL apps by default, and the 11 legacy cues were last rendered by infra-cli, so a
   full re-render may diff heavily. Recommend **platform-component-only** render for the
   cutover; defer full-fleet re-render.
3. Commit re-rendered `k8s/platform/` + `platform.cue` + `platform.toml` in `../infra`.
   (Legacy apps still carry `parts.#UseKeel` — inert annotations without the Keel
   controller; they render fine on v0.3.19. Leave them.)
4. **Reconcile live (gated on chakrit):** apply `k8s/platform/` to move `:latest` →
   `:<pinned>`. infra agent executes.

## Phase 3 — Flux bootstrap (Slice 2) · DEFERRED

`flux.platform` installs controllers; `flux.cue` (OCIRepository + Kustomization → published
artifact); decide the OCI-URL convention + GHCR pull secret. Until then the platform tree is
applied directly (gated), not reconciled.

## Open decisions (chakrit to drive)

1. **Render scope for the cutover** — platform-component-only (recommended) vs full-fleet
   re-render now (churns every legacy app's `k8s/`).
2. **Module-path default style** — repo-address (`github.com/prod9/infra`, current default)
   vs domain (`stage9.dev`).
3. **hello-world default** — in the pre-checked `Defaults` set, or an optional component?
4. **`../infra` defaults** — confirm: leave its existing `defaults/` untouched (don't ship
   baseline defaults over it). Assumed yes.
