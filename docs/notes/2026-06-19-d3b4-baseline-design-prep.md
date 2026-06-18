# D3b-4 baseline — design prep (derived from real `infra`)

**Status:** proposal, parked for chakrit's confirmation · drafted 2026-06-19 during an
autonomous (AFK) run, after D3b-3a/3b landed. D3b-4 itself was **not** implemented: it needs
the option-matrix / CUE-vs-DSL split confirmed (open question #8), and the `settings.toml`
migration is cross-repo + attended-only. This note extracts the facts from `../infra` so the
confirmation is a quick yes/no rather than a discovery exercise.

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

## Proposed baseline file set + `[ops.vars]` (CONFIRM)

Mapping the above onto the D3b-2 filename convention and the generic-vars model:

```
baseline/
  cert-manager.platform              # download upstream cert-manager.yaml, emit it
  nginx-gateway@experimental.platform # experimental manifests (TCPRoute support)
  nginx-gateway@stable.platform       # stable manifests
  flux.platform                       # NEW — Flux seed (source + kustomize controllers)
  engine.platform                     # NEW — engine install (scope TBD)
```

Default `[ops.vars]` the embed ships (versions live in the download URLs as `\(var)`):

```toml
[ops.vars]
cert_manager_version      = "v1.20.2"
nginx_gateway             = "experimental"   # choice: experimental | stable
nginx_gateway_version     = "v2.6.0"
gateway_api_version       = "v1.5.1"
nginx_gateway_firewall_id = "11222746"
nginx_gateway_toleration  = ""
```

`nginx-gateway` as a **choice group** (not a `+experimental` toggle) matches the roadmap's
lexically-first-default note (`experimental` sorts before `stable`, so the unset default is
`experimental` — which is what `infra` runs today). Confirm choice-vs-toggle; if `stable`
should be the safe default, D3b-2's `Select` needs an explicit default marker (it has none
yet).

## CUE vs DSL split (open question #8 — CONFIRM)

- **DSL (`baseline/*.platform`, foreign installs):** cert-manager, nginx-gateway (+ its CRDs
  and the gateway-api CRDs), Flux seed, engine. Pinned by version-in-URL, patched as needed.
- **CUE (`apps/*.cue`, ours):** `cluster-issuer.yaml`, namespaces, RBAC, the `Gateway`/
  `GatewayClass` resources, the platform Deployment. These already exist as CUE in `infra`.

Boundary question: the gateway-api CRDs and nginx-gateway CRDs — one `.platform` each, or
folded into `nginx-gateway@*.platform` as extra `download`+`emit` steps? Folding keeps one
component dir (`k8s/nginx-gateway/`) matching today's layout; confirm.

## Open confirmations before authoring D3b-4

1. **argocd** — drop from the baseline entirely (Flux replaces it), or keep an argocd
   `.platform`? The pull-based-GitOps ADRs point to Flux, so this note assumes drop.
2. **Choice vs toggle** for `nginx-gateway`, and whether `experimental` or `stable` is the
   safe unset default.
3. **CUE/DSL boundary** above, incl. the CRDs folding question.
4. **`flux.platform` / `engine.platform` scope** — these have no `infra` precedent to mirror;
   they need their own design (which controllers, which engine, versions).
5. **Migration mechanics** (cross-repo, attended): fold `settings.toml` → `platform.toml`
   per the table, delete `settings.toml`, then dogfood — `ops render` the authored directives
   and diff against `../infra/k8s/{cert-manager,nginx-gateway}`.

Once 1–4 are confirmed, authoring the directives + default `[ops.vars]`, the `go:embed`, the
bootstrap write-path hook, and the `ScanOptions` prompts is mechanical and in-envelope; the
migration (5) stays attended.
