# GitOps Platform Build Plan — CI + `cue export` + Flux (OCI)

**Status:** build brief for agents. Folded into the repo from the original Downloads draft
and corrected against the 2026-06 design walk (see
[`config-allocation.md`](config-allocation.md) and [`platform.md`](platform.md)).
**Date:** 2026-06-13 (folded 2026-06-14) **Owner:** Chakrit / P9

> **Renderer superseded (2026-06-16):** this brief predates the renderer decision. The
> renderer is now **`cue export` over infra-defs** + a Go multi-doc emitter, with
> third-party installs adapted by the [manifest patch DSL](manifest-patch-dsl.md) — **not
> timoni**. See the
> [renderer ADR](../decisions/2026-06-16-renderer-cue-export-not-timoni.md). The Flux +
> OCI + pull-based mechanics below stand unchanged; read `timoni build` as `cue export`
> throughout. The OCI artifact still holds **rendered manifests** (Flux's
> kustomize-controller applies them; it never renders CUE).

---

## 1. Goal

Pull-based GitOps for Kubernetes, CUE-native via timoni, no inbound cluster creds.
Replaces current Keel.sh tag-watching with full desired-state reconciliation.

## 2. Architecture (one sentence each)

- **Infra repo (CUE/timoni module):** single source of truth for desired state — image
  tags, components, env, replicas, etc.
- **CI:** validates CUE, **renders it (`timoni build`)**, pushes the **rendered
  manifests** as an OCI artifact. Never touches the cluster (no kubeconfig in CI).
- **Registry (OCI):** holds the rendered config artifact. Source the cluster pulls from.
- **Flux (in-cluster):** `OCIRepository` watches the artifact tag; kustomize-controller
  applies/prunes the full manifest set continuously (drift correction).
- **timoni:** the CUE module format + renderer, invoked by **CI** (not Flux).

```
dev -> infra repo (CUE) -> CI (validate, timoni build, push rendered OCI) -> registry
                                                                              |
                                          Flux OCIRepository (poll) -> apply/prune cluster
```

## 3. End-user (dev) flow

1. Dev edits CUE in infra repo (image tag + any components/env), pushes.
2. CI validates CUE, runs `timoni build` → pushes the rendered manifests as a new OCI
   artifact.
3. Flux `OCIRepository` polls registry, notices change.
4. Flux reconciles → applies/prunes full manifest set (continuous, drift-corrected).

Dev only ever touches the CUE repo. App image build is a **separate** pipeline (app repo);
the matching **immutable** image tag must exist before/with the CUE change (design the
deploy flow to couple them so it can't reference an unbuilt image).

## 4. Build tasks (for agents)

### 4.1 timoni module
- [] Scaffold timoni module from existing CUE export (`timoni mod init` or port existing).
- [] Parameterize: image tag, component toggles, env, namespace, replicas.
- [] Verify `timoni build` produces expected manifests locally (diff against current k8s
  state).
- [x] Tag strategy: **both layers** — config artifact = **moving** per-env tag (Flux
  follows); app image refs *inside* = **immutable** versions/digests.

### 4.2 CI pipeline (infra repo)
- [] On push: `cue vet` / `timoni mod vet` — fail fast on invalid CUE.
- [] `timoni build` → push the **rendered manifests** as
  `oci://<registry>/<artifact>:<env>` via a registry token (write-only, NOT cluster
  creds).
- [] Tag policy: push to the per-env **moving** tag so Flux's `OCIRepository` catches it.
- [] Confirm: no kubeconfig / cluster credentials anywhere in CI.

### 4.3 Flux install + config
- [] Bootstrap Flux (source-controller + kustomize-controller; OCI support). No
  helm-controller (Helm banned).
- [] `OCIRepository`: registry URL, moving per-env tag, `interval`, registry **read**
  secret (held by cluster, outbound only).
- [x] Reconciler = **kustomize-controller consuming the rendered manifests** (no native
  timoni-Flux controller exists).
- [] Set `prune: true` for full GitOps delete-on-removal.

### 4.4 Keel cutover
- [] Inventory workloads currently managed by Keel.
- [] Move image-tag selection into the deploy flow (platform writes the immutable tag into
  CUE).
- [] Migrate workload-by-workload; **retire Keel** for anything Flux owns (they fight over
  the image field otherwise — Flux reverts Keel).
- [] Decommission Keel once migrated. (Local deploy is gone — it depended on Keel.)

### 4.5 Secrets / RBAC
- [] Registry read secret for Flux (outbound pull).
- [] Flux controller ServiceAccount RBAC scoped to managed namespaces.
- [] App-repo CI: registry write secret only.
- [] Workload secrets: **platform-pull init-container** — pulls values from the platform
  API at pod start (outbound), values stay in platform. No SOPS/ESO/controller.

## 5. Open decisions — resolved in the design walk

- [x] **Tag strategy:** both — moving config tag + immutable app images (§4.1).
- [x] **timoni-on-Flux wiring:** rendered-manifest pattern via kustomize-controller (no
  native controller exists).
- [x] **App image coordination:** design the deploy flow to couple build+config so an
  unbuilt image can't be referenced; optional infra-CI validate.
- [x] **Single vs multi cluster:** single home cluster for v2; multi-cluster = phase 2.

## 6. Explicit non-goals

- No CI-to-cluster apply (preserves no-inbound-creds).
- No CRD authoring / custom controller.
- No sidecar on a repo-server (this is Flux, not ArgoCD).
- No committed rendered YAML (the OCI artifact is the rendered package).

## 7. Acceptance criteria

- [] Dev pushes CUE change → within poll interval, cluster reflects it, no manual step.
- [] Removing a resource from CUE → Flux prunes it.
- [] Manual `kubectl edit` to a managed resource → Flux reverts (drift correction).
- [] No cluster credentials exist outside the cluster.
- [] Keel retired (or strictly scoped); no image-field fighting.
