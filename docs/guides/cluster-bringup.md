# Cluster bring-up — fresh cluster to a live platformv2 baseline

Runbook for standing up a **brand-new cluster** on the platformv2 delivery path
(Flux GitOps + committed-literal images). Written for the infra-executing agent; the
platform session is the design authority. Scope is the baseline only — app onboarding and
legacy teardown are later phases, out of scope here.

> Authority: platformv2 supersedes the legacy ArgoCD/Keel path fleet-wide. Legacy-grounded
> objections do not bind this runbook; when it conflicts with old-cluster practice, this
> runbook wins. Questions go back to the platform session, not to legacy artifacts.

## Prerequisites (before any step)

- A fresh Kubernetes cluster with **amd64 nodes** (`publish` ships amd64 images) and a
  working `kubectl` context. Provider per the executing agent's tooling (prior clusters
  are Linode; the nginx-gateway LB expects a Linode firewall — capture its **firewall id**).
- A **new GitHub repo whose name contains `infra`** (framework discovery matches the
  directory name glob — `framework/infrabase.go` `hasInfraName`). Clone it; rename the
  remote to `gh`.
- ghcr.io credentials able to **pull** the infra image (read-only PAT is enough for the
  cluster; the operator's local docker creds handle the push side).
- DNS control for the two ingress hosts (`PLATFORM_HOSTNAME`, `FLUX_HOSTNAME`).
- A `platform` binary (>= the current `main`) on the operator machine driving init/publish.

## 1. Scaffold the repo

In the fresh clone (already a git root — init validates this, it never runs `git init`):

```sh
ALWAYS_YES=1 platform init <maintainer> <email> <github.com/ORG/REPO-infra> <CUE_MOD_PREFIX>
```

Positional args feed the value prompts in order; `ALWAYS_YES=1` only auto-answers the
final yes/no confirm. `CUE_MOD_PREFIX` is the CUE module path (first segment must contain
a dot, e.g. `prodigy9.co`) — asked greenfield only; an existing `cue.mod` is read instead.

This writes the full baseline: `platform.toml` (strategy `rolling`, default `[vars]`
version pins), `apps/` (cert-manager, flux, flux-sync, nginx-gateway-exp, platform),
`defaults/basics.cue`, `cue.mod`, and the `platform` launcher.

Re-running init later with `--force` replaces framework-owned files **including any
secrets you wired into them** (the flux-sync HMAC token, the basics registry creds) —
that's what "replace existing files" means. Commit before a `--force` re-scaffold and
restore the wired values from git after. Only `platform.toml` merges surgically.

## 2. Wire values (edit before first render)

| File                   | What to set                                                            |
|------------------------|------------------------------------------------------------------------|
| `platform.toml` `[vars]` | `PLATFORM_HOSTNAME` / `FLUX_HOSTNAME` — this cluster's ingress hosts. `NGINX_GATEWAY_FIREWALL_ID` — this cluster's Linode firewall id. `NGINX_GATEWAY_RESERVED_IPV4` — the cluster's reserved LB IPv4; **must be set before anything from `k8s/nginx-gateway-exp/` first applies** — the Linode CCM honors the annotation only at Service creation, retrofitting does nothing, the fix is delete/recreate. No reserved IP → delete the reserved-ipv4 directive from `apps/nginx-gateway-exp.platform` instead (empty-value API behavior is unverified). Leave version pins alone. |
| `defaults/basics.cue`  | `#registry_username` / `#registry_password` — the ghcr **pull** creds (committed placeholders are empty). |
| `apps/flux-sync.cue`   | `webhookToken` `#data: token:` — a fresh random HMAC secret (plaintext; `#Secret` base64-encodes). Generate with `openssl rand -hex 32`. |

Commit the lot. This repo is the delivery record — the cluster runs what this git says.

## 3. Bootstrap apply (no server, pull loop not yet live)

```sh
platform render
```

renders `apps/` → `k8s/<component>/…`. Apply with `kubectl apply --server-side` (the
gateway-api/NGF CRDs exceed the 256KiB `last-applied-configuration` annotation limit, so
client-side apply fails on them; server-side is safe for the whole bootstrap, add
`--force-conflicts` on re-runs) in this order, waiting for CRDs to establish between
steps:

1. `k8s/nginx-gateway-exp/gateway-api-crds.yaml` + `nginx-gateway-crds.yaml`
2. `k8s/cert-manager/` — wait for the webhook deployment to be Ready
3. `k8s/nginx-gateway-exp/nginx-gateway.yaml`
4. `k8s/flux/` — wait for Flux CRDs (`ocirepositories`, `kustomizations`, `receivers`)
5. `k8s/flux-sync/` — the OCIRepository + Kustomization + webhook Receiver/Secret/Route
6. `k8s/platform/`

A second idempotent `kubectl apply` pass over the whole tree is an acceptable
convergence check; anything still failing is a real error, not ordering.

## 4. Gateway + TLS (operator config — NOT shipped by the baseline)

The baseline's routes reference a Gateway named **`nginx` in namespace `gateway`** that
the scaffold does **not** create. Create it (and its namespace) with:

- listeners covering `PLATFORM_HOSTNAME` and `FLUX_HOSTNAME`;
- `allowedRoutes` admitting the `flux-system` and `platform` namespaces;
- TLS via cert-manager (issuer is likewise operator config today).

Point DNS for both hostnames at the gateway's LoadBalancer IP.

## 5. First publish — light the pull loop

From the infra repo, on the operator machine (ghcr push uses local docker creds):

```sh
platform publish
```

This renders the manifest tree, packs it into a scratch image, and pushes
`ghcr.io/ORG/REPO-infra:latest`. Flux's OCIRepository pulls it and the Kustomization
applies it — from this point the cluster follows the published image; `kubectl apply` is
bootstrap-only.

## 6. Hand-wire the GitHub webhook (deferred-to-`srv` gap)

Platform does not yet configure the GitHub side. On the GitHub org/repo that owns the
ghcr package, create a webhook:

- URL: `https://<FLUX_HOSTNAME>` + the Receiver's path — read it from the cluster:
  `kubectl -n flux-system get receiver infra -o jsonpath='{.status.webhookPath}'`
- Content type json; secret = the HMAC token from step 2; event: **`registry_package`**.

The webhook is the primary, near-instant reconcile trigger; the OCIRepository's 10m poll
is only the dropped-webhook fallback.

## 7. Verify end-to-end

1. Commit a trivial visible change (e.g. a manifest label) to the infra repo; `platform publish`.
2. GitHub fires `registry_package` → Receiver validates → OCIRepository re-pulls →
   Kustomization applies. Confirm the change lands on-cluster within seconds (not the 10m
   poll): `kubectl -n flux-system get ocirepository,kustomization` timestamps move.
3. `https://<PLATFORM_HOSTNAME>` serves the vanity redirect; certs valid.

Baseline is live when all three hold.

## Known gaps / decision points

- **Gateway + issuer in the baseline?** Today both are operator config (step 4). If every
  bring-up repeats them verbatim, propose promoting them into the scaffold baseline —
  that's a platform-repo decision, raise it there.
- **GitHub webhook auto-config** — deferred to the platform server (`srv`); hand-wired
  until then.
