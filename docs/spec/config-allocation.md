# Config Allocation Map — platformv2

**Status:** intended allocation; implementation is partial. Companion to
[`architecture.md`](architecture.md) and [`lua-render.md`](lua-render.md); frozen rulings
in [`../decisions/`](../decisions/). Distilled from the 2026-06 design walk and the
2026-11-25 rescope of `platform` to a CLI that renders Lua
([server-removed-platform-is-a-cli](../decisions/2026-11-25-server-removed-platform-is-a-cli.md),
[cue-dropped-for-lua-render](../decisions/2026-11-25-cue-dropped-for-lua-render.md)).

The job of this document: **exactly one owner per config kind.** Nothing settable in two
places, no gaps. The Flux pivot left us with several surfaces — this map is the discipline
that stops them overlapping.

## The invariant

**No cluster credentials exist outside the cluster.** Everything is pull-based: the
cluster reaches out (Flux pulls OCI artifacts); nothing reaches in. `platform` is a CLI
that builds images and pushes them to a registry, so it holds no cluster credential at
all; a cluster credential in use is the operator's own, as for the manual baseline seed.
Every allocation below is chosen to preserve this.

## Surfaces, as layers

Top (closest to the human) to bottom (closest to the metal). One owner each.

- **`platform.toml`** (in each source repo, git) — per-repo build metadata: modules,
  release strategy, arch targets, and for an infra repo its `[vars]` and `[rocks]` pins.
  Changes with the code.
- **`infra/` repo|path** (Lua, git) — the cluster's desired state, written as `apps/*.lua`
  against the `infra-lua` library: app image refs (committed literals the operator
  hand-edits — platform never rewrites these files), components, replicas, and per-env
  instances separated by k8s namespace. The thing `platform` renders, on its built-in Lua
  VM. No committed rendered YAML.
- **`tf/` repo|path** (OpenTofu, git) — cloud/DNS provisioning; not a platform env list.
  Applied **manually, locally**.
- **OCI registry** — app images (digest-pinned to dodge stale cache), the rendered config
  artifact (moving tag; the infra repo's git history is the record), and published rocks.
  The source Flux and `render` pull from.
- **Flux** (in-cluster: source-controller + kustomize-controller) — reconciles the config
  artifact onto the cluster; applies/prunes; drift correction. No Argo. No Helm.
- **cluster baseline** — Flux, cert-manager, NGINX Gateway with the Gateway API
  definitions, and the gateway, issuer and Flux-sync apps. Shipped as Lua templates in the
  `infra-lua` rock and written into `apps/` by `platform init` through the `Infra`
  framework; from then on the repo owns the files. Seeded once (manual), then
  Flux-reconciled — except Flux's own lifecycle, never self-managed. See
  [`scaffolding.md`](scaffolding.md) and
  [dsl-removed-for-lua-templates](../decisions/2026-11-25-dsl-removed-for-lua-templates.md).

## Allocation table

| Config kind                           | Owner             | Form                               |
| ------------------------------------- | ----------------- | ---------------------------------- |
| How to build a module                 | `platform.toml`   | TOML in source repo                |
| Render inputs (`[vars]`)              | `platform.toml`   | TOML → `platform.vars` in Lua      |
| `infra-lua` version (`[rocks]`)       | `platform.toml`   | TOML pin → rock fetched from ghcr  |
| Authorization (who may deploy)        | GitHub            | infra-repo push permission         |
| Desired state (image, replicas, env)  | `infra/`          | Lua (`apps/*.lua`) → OCI           |
| App image (the container)             | OCI registry      | digest-pinned tag                  |
| Config artifact (Flux source)         | OCI registry      | moving tag (git = record)          |
| `infra-lua` rock                      | OCI registry      | one-layer image per version        |
| What's deployed where, drift          | Flux              | reconciles from OCI                |
| Cluster baseline (Flux/CM/NGF)        | `infra/`          | template copies in `apps/`         |
| Cloud / DNS                           | `tf/`             | OpenTofu                           |

Where secret values live is **open**: the platform server owned them, with pods pulling
them through init-containers, and nothing in this map owns them now.

## `platform.toml` keys for render

### `[rocks]`

A new top-level table that maps a rock name to the version `render` uses:

```toml
[rocks]
infra-lua = "<version>"
```

The `Infra` framework's dependency step fetches each pinned rock from ghcr into
`$XDG_CACHE_HOME/platform/rocks/<name>/<version>/`, and a version already there is not
fetched again. `platform init` writes the pin, choosing the newest published version on a
fresh repo (intended). The pin is the only place a rock version is chosen; `infra-lua` is not built
into `platform`. Fetch, cache and credentials: [`lua-render.md`](lua-render.md).

### `[vars]` — how they reach Lua

`[vars]` is a generic open table whose values keep their TOML type (string, integer,
boolean) ([`conf/conf.go`](../../conf/conf.go), `Vars`). `render` injects the table into
every app as `platform.vars`; it is the only path a value takes from `platform.toml` into
Lua. A template declares the vars it reads and their defaults at the top of the file —
`local vars = platform.vars {CERT_MANAGER_VERSION = "v1.20.2"}` — and `platform init`
writes each declared var with its default into `[vars]`, keeping values already set. The
declaring call and how values meet defaults are [`lua-render.md`](lua-render.md). Seeding
`[vars]` from the templates at init is **intended**.

Var names reach Lua as written in `platform.toml`.

## Repos & artifacts

- **source repo** — app code + `platform.toml`. `platform publish` (Dagger) builds the
  immutable image.
- **`infra/`** — the desired-state Lua. `platform` renders it on its Lua VM → the `k8s/`
  manifest tree → pushed as the OCI config artifact under a **moving** tag (git history is
  the record). App image refs *inside* are committed literals, digest-pinned to dodge
  stale cache.
- **`infra-lua`** — the Lua library and the baseline templates, packaged as a LuaRock and
  published to ghcr as a one-layer image by `platform publish`
  ([frameworks.md](frameworks.md), LuaRock).
- **`tf/`** — OpenTofu cloud/DNS provisioning; manual local apply. Not a platform env
  list.

## Delivery flow (where the surfaces meet)

1. `platform publish` builds and pushes the app image (digest-pinned).
2. The operator **hand-edits** the app-image ref in the `infra/` Lua and commits —
   platform never rewrites those files. The gate is GitHub push permissions on the infra
   repo.
3. `publish` (infra is a framework) renders the Lua, builds the
   manifest tree into a `FROM scratch` image and pushes it to the moving tag — the
   ordinary publish path, no bespoke OCI pusher. See
   [infra-publishes-as-plain-image-retire-oras](../decisions/2026-07-05-infra-publishes-as-plain-image-retire-oras.md).
4. Flux's `OCIRepository` follows the tag, extracts the layer via `layerSelector` →
   reconciles → pods run the pinned image. The publish fires GitHub's `registry_package`
   webhook → the GitHub→Flux `Receiver` (one per cluster, listing each reconciled
   namespace, one wildcard `OCIRepository` entry per ns) triggers a near-instant
   reconcile; the
   `OCIRepository` poll interval is only the fallback when the Receiver misses a delivery.

No step pushes into the cluster. The gate is the git commit; everything after is pull.
