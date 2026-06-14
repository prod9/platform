# platformv2 — design notes (running log)

> **CLOSED 2026-06-14.** The design walk is complete. Final state lives in
> [`../spec/platform.md`](../spec/platform.md),
> [`../spec/config-allocation.md`](../spec/config-allocation.md), the build brief
> [`../spec/gitops-build-plan.md`](../spec/gitops-build-plan.md), and the ADRs in
> [`../decisions/`](../decisions/). What follows is the exploration journey — including
> the mid-stream ArgoCD→Flux pivot — kept as history, not current truth.

Started 2026-06-12. Running capture of the v2 design conversation. Disposable;
settled output lands in [`../spec/platform.md`](../spec/platform.md) and rulings
in [`../decisions/`](../decisions/). When a fork closes, promote the ruling to a
decision/spec and trim it here.

## Goal

Evolve `platform` from a build CLI into the next-gen build+deploy backbone:
retire BuildKite, build images server-side, encode K8s deploys via Argo
(GitOps), bootstrap/manage clusters, and bind multiple source + infra repos
under a **Project**. Supersedes `PLANS.md` #3 (infra-cli fold-in as a CLI) —
we go server-side instead of CLI-only.

## Stability findings — Dagger on K8s

- Engine-on-K8s is **supported**: Helm chart, Dagger Engine as a DaemonSet per
  node, local NVMe build cache. Two shapes — persistent nodes (keep cache) or
  autoscaled nodes (lose cache on de-provision; PVs mitigate).
- Only experimental surface: `_EXPERIMENTAL_DAGGER_RUNNER_HOST` / `kube-pod://`
  — the scheme a client *outside* the cluster uses to reach an in-cluster
  engine. Experimental-in-name for years while used in prod. Run the build
  *inside* the cluster (Job/pod beside the engine) and the hop never crosses it.
- Dagger's "third-gen" pattern runs the BuildKite agent / GHA runner *as a
  service beside the engine* — they replace the **executor**, not the CI
  **control plane**.
- Refs: [Dagger K8s docs](https://docs.dagger.io/reference/deployment/kubernetes/),
  [On-demand engines w/ Argo CD + Karpenter](https://dagger.io/blog/argo-cd-kubernetes/).

## Reframe — "get rid of BuildKite" is two decisions

- **Runner** (consume a job, run the build) — Dagger replaces cleanly, no
  experimental anything.
- **Control plane** (webhook intake, queue, history, retries, secrets, UI) —
  undifferentiated platform we'd build *and operate forever*. The real question
  is: build it, or borrow K8s + Argo as the control plane.

## Decision map

Statuses below reflect chakrit's reply (2026-06-12); detail in "Reply digest".

| # | Fork | Status |
|---|------|--------|
| 1 | Control-plane scope | **decided: build it** — real server + UIs. Justified by RBAC/identity, not build-orchestration. |
| 2 | Where builds execute | **decided: in-cluster Dagger engine.** ⚠ open risk: per-build resource is engine-opaque, not surfaced to kube (see digest). |
| 3 | Event source | **decided: GH webhooks + platform-CLI→server trigger.** No GH Actions. Needs auth layer email→gh→k8s SA→argo. |
| 4 | Deploy mechanism | **decided: platform server does the git dance** (CLI fallback shells out via dagger). Argo reconciles. |
| 5 | "Project" model | **decided: yes** — Project is the unit of work. |
| 6 | Cluster bootstrap | **decided: CUE manifests applied to k8s + basic checks.** Bonus: DNS via Cloudflare API. infra-cli folds in here. |

## Fork 1 — control-plane scope (OVERTURNED — see Reply digest)

> **Overturned 2026-06-12.** chakrit wants the control plane *built* — that's the
> point, not overhead. Goal is consolidation + identity, not avoiding undifferentiated
> work. The proposal below (borrow K8s+Argo, minimal server) is kept as record only.


**Proposal:** don't build the control plane. platform = a server-shaped library
with a thin event front-door.

- A small long-running service does only: receive trigger → resolve Project →
  launch build as a K8s Job against the in-cluster Dagger engine → on success,
  image-tag commit to the infra repo. Holds creds + RBAC (= the minimum the
  spec already blesses).
- All *logic* (CUE-gen, discover, build, version, rollout) stays in the library
  the CLI already wraps. The server is a **second caller of the same spine**,
  not a new brain.
- History / logs / retries: lean on K8s (Jobs = history, `kubectl logs`, Job
  backoff) + Argo's UI for rollout state. No dashboard until something forces it.

**Why:** the spine (CUE-gen → build → version → rollout) is the differentiated
value; 4 of 5 tools already converge there. The control plane is what everyone
rebuilds and nobody enjoys operating.

**Counter-pull:** ideas #4/#5 (multi-repo, multi-cluster, new-cluster setup)
*sound* like they need an orchestrator. Claim: they don't yet — bootstrap is a
generate-and-commit op (same shape as deploy); multi-repo is a data-model
problem (the Project). Both ride on `minimal server + Project model`; no
queue/scheduler/UI.

**Still open:** the scope call itself; and the "K8s + Argo *as* the control
plane" substitution.

## Reply digest — chakrit (2026-06-12)

**North star: one streamlined tool.** Knowledge is scattered across the 5 tools; the
*consolidation itself* is the goal. Overrides the earlier "don't rebuild undifferentiated
platform" framing — owning the whole way-of-work in one place is the win.

**The server's real spine is identity/RBAC, not build-orchestration.** Drivers:
- Self-serve project setup (currently cumbersome).
- Prune access cleanly when people leave.
- Kill the root/superadmin creds scattered across the current setup; replace with
  fine-grained controls.
- Identity chain to map + broker: **work email → GitHub account → k8s service account →
  Argo account.** This auth layer gates webhooks *and* CLI-triggered builds.

Build / deploy / bootstrap / DNS hang off `Project + identity`. Control plane ships
**with UIs**.

**Helm: banned** (magic; breaks human-traceability — logged in personal CLAUDE.md).
Knock-on: can't use Dagger's Helm chart to install the engine either → the engine is
installed via our own CUE manifests (#6), same path as all other infra. The anti-Helm
stance and "CUE manifests for k8s" converge: the Dagger engine is just more managed infra.

**⚠ Fork-2 risk — Dagger resource model vs. "manage purely through kube".** Half-right:
- ✅ Scale-out = add/upgrade nodes; DaemonSet lands an engine per node. No separate build
  farm. Matches the mental model.
- ❌ Builds run *inside* the engine pod (buildkit), not as separate kube pods. Per-build
  CPU/mem and Dagger service containers are **not** surfaced to kubectl — kube sees one
  engine pod; isolation + limits are buildkit's job. Nothing per-build in `kubectl get pods`.
- Settle before #2 closes: accept engine-opaque execution, or run builds as ephemeral K8s
  Jobs (kube-visible + per-build limits, but loses Dagger's cache/DAG). Current lean:
  accept opaque builds, get per-build observability from platform's own UI, not kubectl.
  TODO: verify exact resource-reporting behavior of the engine on K8s.

## Open questions

- Fork 2: engine-opaque builds vs. ephemeral K8s Jobs — which, and is partial kube
  visibility acceptable? (verify Dagger reporting first)
- Identity broker: where do GitHub↔k8s-SA↔Argo mappings live, and what's the source of
  truth — the Project, or a separate identity store?
- UIs: surface/scope of the control-plane UI (build history, project admin, access mgmt) —
  parked until the spine is settled.
- DNS via Cloudflare API: in-scope for bootstrap (#6) or a later add-on?
