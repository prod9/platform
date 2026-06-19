# D3b-4 baseline — design prep (derived from real `infra`)

**Status:** confirmed by chakrit 2026-06-19 (see Decisions). Drafted during an autonomous (AFK)
run after D3b-3a/3b landed; D3b-4 itself not yet implemented. This note extracts the facts from
`../infra` and now records the confirmed design.

## Decisions (chakrit, 2026-06-19)

1. **argocd** — keep a **reference** `argocd.platform` (version-pinned), not the active delivery
   tool (Flux is). Open micro-point: applied-by-default vs an opt-in toggle — see below.
2. **nginx-gateway** — **experimental only.** Stable lacks a feature we require (TCPRoute) and
   "doesn't even work." So there is **no choice group**: a single plain `nginx-gateway.platform`
   pinned to the experimental manifests. The whole experimental-vs-stable matrix drops.
3. **engine = Dagger engine, authored in CUE, not DSL.** Per the
   [2026-06-12 design](2026-06-12-platformv2-design.md) (Helm banned → no Dagger Helm chart →
   the engine installs via our own CUE manifests as a DaemonSet, "just more managed infra").
   So **drop `engine.platform`**; the engine is a CUE app, ours.
4. **Versions live in `[ops.vars]`**, string-interpolated as `\(var)` into the directive
   `download` URLs (confirmed).
5. **Migration is minimal.** The tooling (argo/flux) needs no migration; nginx-gateway has
   existing guides/skills for the ingress→gateway-api move; cert-manager is ~a no-op. Workloads
   stay put, CUE is unchanged, namespaces/yaml are same-ish (formatting diffs). So D3b-4's
   "migration" is just folding the `settings.toml` values into `platform.toml` `[ops.vars]`;
   the real cutover (delete Buildkite once Flux + platform CI are up) is later (Slice 2).

## What the real `infra` actually ships

`../infra/settings.toml` (the knobs to migrate into `[ops.vars]`):

| settings.toml key                  | value                                   | fate                          |
|------------------------------------|-----------------------------------------|-------------------------------|
| `maintainers`                      | `["Chakrit … <chakrit@prodigy9.co>"]`   | → `platform.toml` `maintainer`|
| `repo.url`                         | `https://github.com/prod9/infra`        | → `platform.toml` `repository`|
| `repo.path` / `repo.revision`      | `k8s` / `main`                          | Flux delivery; not a DSL var  |
| `argocd.version`                   | `v3.4.1`                                | argocd → Flux: drop? (confirm)|
| `argo_project` / `generate_argo` / `kube_apply` | empty / `false` / `false`  | legacy argocd delivery; drop? |
| `namespace_prefix`                 | `""`                                    | drop? (unused) (confirm)      |
| `cert_manager.version`             | `v1.20.2`                               | → `[ops.vars]`                |
| `nginx_gateway.version`            | `v2.6.0`                                | → `[ops.vars]`                |
| `nginx_gateway.gateway_api_version`| `v1.5.1`                                | → `[ops.vars]`                |
| `nginx_gateway.experimental`       | `true` (comment: required for TCPRoute) | → **choice** (see below)      |
| `nginx_gateway.toleration`         | `""`                                    | → `[ops.vars]`                |
| `nginx_gateway.annotations[...firewall-id]` | `"11222746"`                   | → `[ops.vars]`                |

Foreign installs currently in `../infra/k8s/` (the `.platform` candidates):

| component       | committed output files                                              |
|-----------------|---------------------------------------------------------------------|
| `cert-manager`  | `cert-manager.yaml` (upstream), `cluster-issuer.yaml` (ours)        |
| `nginx-gateway` | `gateway-api-crds.yaml`, `nginx-gateway-crds.yaml`, `nginx-gateway.yaml` |

## D3b-4b decisions + progress (chakrit, 2026-06-19)

- **Write path = `platform init`** (landed). A command distinct from `bootstrap`: git-inits the
  target, writes `platform.toml` (with default `[ops.vars]`) + the embedded `baseline/*.platform`.
  Does not write the app build script / CI pipeline.
- **Options are pure DSL files + a generic checkbox picker** (landed). No per-component Go code:
  `platform init` runs one `baseline.ScanOptions` loop, presenting each toggle as a yes/no
  checkbox (pre-checked from the current `[ops.vars]` value) and each choice as pick-one;
  `--force` keeps shipped defaults for CI.
- **Always-on** (plain `name.platform`, no checkbox — "clearly needed for a functioning
  cluster"): cert-manager, flux, engine. (cert-manager + flux landed; engine is CUE, deferred.)
- **Optional toggles:** **argocd** (`argocd+argocd.platform`, default **off** — reference
  install; landed) and **NGF-experimental** (default **on** — pending, see below).

### NGF directive — recipe + remaining blocker

**Source of truth:** `prod9/infra-cli/cmd/nginx_gateway_cmd.go` (the cli platform replaces). It
**downloads plain manifests** (no Helm at render time — the manifests are pre-baked upstream):

| step | URL | emit |
|------|-----|------|
| Gateway API CRDs | `…/kubernetes-sigs/gateway-api/releases/download/\(gateway_api_version)/experimental-install.yaml` | `gateway-api-crds.yaml` |
| NGF CRDs | `…/nginx/nginx-gateway-fabric/\(nginx_gateway_version)/deploy/crds.yaml` (raw) | `nginx-gateway-crds.yaml` |
| NGF controller | `…/nginx/nginx-gateway-fabric/\(nginx_gateway_version)/deploy/default/deploy.yaml` (raw) | `nginx-gateway.yaml` |

(Experimental gateway-api install is required for TCPRoute — hence `experimental-install.yaml`.)
Patches on the NGF controller manifest (cli does these via `pipelines.EditYAML`):

- **serverTokens=off** — `select ".kind" "NginxProxy"` → `set ".spec.serverTokens" "off"` (NGF
  2.5.1 bug workaround).
- **firewall annotation** — append a StrategicMerge entry to
  `NginxProxy.spec.kubernetes.service.patches`, carrying
  `metadata.annotations."service.beta.kubernetes.io/linode-loadbalancer-firewall-id"` =
  `\(nginx_gateway_firewall_id)`. NGF v2's CRD has no `service.annotations` field — the patches
  list is the only hatch.
- *(cli also has conditional toleration + Deployment→DaemonSet; both off in current infra —
  defer.)*

**Verified working** for this recipe: `select PATH VALUE` (doc filter), escaped quotes in
tokens, quoted keys, and the new auto-vivify `set` (builds `patches[0]…` from nothing).

**Remaining blocker — string-forcing.** `set`'s `scalar()` runs the value through
`yaml.Unmarshal`, so `\(nginx_gateway_firewall_id)` = `"11222746"` coerces to **int**. But a
k8s annotation value must be a **string** (`annotations` is `map[string]string`) — an int there
is invalid. The DSL has no way to force a string-typed scalar for a numeric-looking value.
Options (a design call): a `set-string` verb; make `scalar()` respect an already-quoted input
(`set … "\"\(x)\""`); or a typed-set syntax. **Needs a decision before the NGF directive can
emit a valid firewall annotation.** Everything else is ready.

### Earlier framing (superseded by the recipe above)

NGF does *not* ship a tarball/release-asset install manifest — but it serves plain YAML from the
repo tree (`deploy/crds.yaml`, `deploy/default/deploy.yaml`) + gateway-api's release assets, so
`download` works without Helm. The "Helm-only → must be CUE" worry is resolved: it stays DSL.

## Baseline file set + `[ops.vars]` (confirmed shape)

Foreign upstream installs → DSL `baseline/*.platform`. Versions interpolated from `[ops.vars]`
into the `download` URLs:

```
baseline/
  cert-manager.platform     # download cert-manager \(cert_manager_version), emit (~no-op)
  nginx-gateway.platform    # gateway-api CRDs + nginx-gateway CRDs + experimental install,
                            #   patched with the firewall-id annotation + toleration
  flux.platform             # NEW — Flux controllers (source + kustomize); version TBD
  argocd.platform           # reference only; download argocd \(argocd_version)
```

Default `[ops.vars]` the embed ships:

```toml
[ops.vars]
cert_manager_version      = "v1.20.2"
nginx_gateway_version     = "v2.6.0"
gateway_api_version       = "v1.5.1"
nginx_gateway_firewall_id = "11222746"
nginx_gateway_toleration  = ""
argocd_version            = "v3.4.1"
flux_version              = "?"          # no infra precedent — pick a current Flux release
```

No `nginx-gateway` choice group (decision #2: experimental-only). The lexically-first-default
note in the roadmap is therefore moot for nginx-gateway.

Ours, authored in CUE (`apps/`) — bootstrap may seed these, but they are not `.platform`:
cluster-issuer, namespaces, RBAC, the `Gateway`/`GatewayClass` instances, the **Dagger engine
DaemonSet** (decision #3), and (Phase B) the platform control-plane Deployment.

## CUE vs DSL split (open question #8 — resolved)

This was prep-note Q3 ("what do you mean?"). Two parts:

- **The boundary** — which manifests we *author ourselves in CUE* (`apps/`) vs *pull-and-patch
  from upstream via the DSL* (`baseline/*.platform`):
  - **DSL (foreign upstream installs):** cert-manager, nginx-gateway (+ its CRDs + the
    gateway-api CRDs), Flux controllers, argocd (reference). Version-pinned in the URL, patched
    where needed.
  - **CUE (ours):** cluster-issuer, namespaces, RBAC, `Gateway`/`GatewayClass`, the Dagger
    engine DaemonSet, the platform control-plane Deployment. Already CUE in `infra` today.
- **The CRDs-folding sub-question** — nginx-gateway upstream ships three files
  (`gateway-api-crds.yaml`, `nginx-gateway-crds.yaml`, `nginx-gateway.yaml`). Author them as
  **one `nginx-gateway.platform`** doing three `download`→`emit` steps (all three land in
  `k8s/nginx-gateway/`, matching today's layout), rather than three separate directive files.
  *Default taken; reversible.*

## Remaining open before authoring

- **flux version** — no `infra` precedent; pick a current Flux release to pin `flux_version`.
- **argocd** — applied-by-default (plain `argocd.platform`) vs opt-in
  (`argocd+enabled.platform` toggle, off by default). Since Flux is the delivery tool and
  argocd is "reference," an off-by-default toggle is the safer read — confirm.
- **CUE baseline bits scope** — is the Dagger engine DaemonSet (and platform Deployment)
  authored as part of D3b-4's embedded baseline, or deferred (engine alongside Phase B)? The
  foreign `.platform` set + the embed/bootstrap mechanism can land without them.

Authoring the directives + default `[ops.vars]`, the `go:embed`, the bootstrap write-path hook,
and the `ScanOptions` prompts is mechanical and in-envelope once the above settle. The migration
(fold `settings.toml` → `platform.toml`, delete it; dogfood `ops render` vs
`../infra/k8s/{cert-manager,nginx-gateway}`) stays cross-repo + attended.
