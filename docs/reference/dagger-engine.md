# Dagger Engine — Capabilities & Deployment Reference

Lookup facts for deploying and connecting to the Dagger Engine, gathered to inform the
in-cluster engine topology (decision #3 in the
[D3b-4 design-prep note](../notes/2026-06-19-d3b4-baseline-design-prep.md)). Verified
against live docs on 2026-06-21. The `_EXPERIMENTAL_*` env vars are explicitly marked
unstable by upstream — re-verify on engine upgrades.

## At a glance

| Item                      | Value                                                            |
|---------------------------|-----------------------------------------------------------------|
| SDK pinned (`go.mod`)     | `dagger.io/dagger v0.20.8`                                       |
| Latest engine + CLI       | `v0.21.7` (engine and all SDKs share one version number)        |
| Our connect call          | `builder/session.go:16` — bare `dagger.Connect(...)`            |
| Today's behavior          | Auto-provisions an ephemeral engine in a local container        |
| Compat mode               | Newer engines can simulate older-engine behavior on upgrade     |

## Concurrency / session model

A **single engine serves many concurrent client sessions** — this is the load-bearing
fact for topology. The engine is a BuildKit-based daemon exposing a GraphQL API:

- Each client connection opens its own `daggerSession` with isolated execution state,
  metadata, telemetry, and `dagql.Server` instance.
- **Regular containers are de-duplicated across the whole engine** (cross-session cache
  reuse, content-addressed by operation digest); **service containers are de-duplicated
  only within a single client session** (prevents cross-talk between clients).
- No documented per-engine session-count limit. Throughput is bounded by node CPU/RAM
  and disk, not by an architectural cap.

Implication: one long-lived engine can back many parallel build sessions. A
DaemonSet/multi-engine fan-out is **not** required for concurrency — it is a *cache
locality and co-location* choice (see topologies).

## Connecting to a pre-provisioned engine

Set `_EXPERIMENTAL_DAGGER_RUNNER_HOST` before the SDK `Connect` call to skip
auto-provisioning and target an existing engine. Companion var
`_EXPERIMENTAL_DAGGER_CLI_BIN` points at a pre-installed CLI binary (skips download).

| Scheme                          | Connects to                                            |
|---------------------------------|--------------------------------------------------------|
| `kube-pod://<pod>?namespace=…`  | A specific k8s pod (params: `context`, `namespace`, `container`) |
| `unix://<socket path>`          | Local UNIX socket (e.g. a shared `/var/run/buildkit`)  |
| `tcp://<addr:port>`             | TCP — **plaintext, no wire encryption**                |
| `container://<name>`            | Runner in a named host container (needs local runtime) |
| `image://<image ref>`           | Starts a runner from an image (needs local runtime)    |

`kube-pod://` and `unix://` (shared volume) are the in-cluster paths. **`tcp://` behind a
plain k8s `Service` is broken** — see the load-balancer pitfall. **platform's chosen path:**
`tcp://` to a StatefulSet pod's stable ordinal DNS via a *headless* Service (not a VIP) — see
the [engine-topology ADR](../decisions/2026-06-21-dagger-engine-statefulset-tcp.md).

## Engine runtime requirements

| Requirement        | Detail                                                              |
|--------------------|--------------------------------------------------------------------|
| Privileges         | `--privileged`, root + `CAP_SYS_ADMIN`; **no rootless mode yet**    |
| Snapshotter        | `overlayfs` required                                                |
| Cache volume       | Mount at `/var/lib/dagger` — without it, severe perf degradation    |
| Node baseline      | ~2 vCPU / 8 GB RAM is the documented starting point                 |
| Disk               | Moderate-to-large NVMe materially speeds builds (artifact cache)    |
| Client co-location | Sessions expect the SDK client on the **same host** as the engine, with matching privileges + working directory (local dirs/sockets must match) |

## Caching

- Cache lives on the engine node's `/var/lib/dagger`. **Ephemeral nodes lose cache on
  de-provision**; persistent volumes preserve it across job churn.
- Multiple clients on one engine share its cache (cross-job reuse on the same engine).
- This is the entire reason upstream's default topology is per-node, not per-cluster.

## Deployment topologies

| Topology                     | Concurrency | Cache reuse        | Routing                       | Caveat                                              |
|------------------------------|-------------|--------------------|-------------------------------|-----------------------------------------------------|
| **DaemonSet** (upstream default) | per-node engine | best on persistent nodes; dies with autoscaled nodes | client → **local** node pod via `kube-pod://` | one engine per node whether used or not             |
| **StatefulSet r=1 + headless Svc** ← *platform's choice* | one shared engine multiplexes all sessions | per-ordinal PVC keeps cache warm | `tcp://<sts>-0.<svc>.<ns>.svc:<port>` (stable ordinal DNS) | single node's resources cap it; scale by adding ordinals + client-side sharding |
| **Deployment, N replicas + Service** | per-replica | fragmented | round-robin Service | **BROKEN** without session affinity — see pitfall; sticky-by-ClientIP pins all to one engine when the client is one pod |
| **On-demand / ephemeral** (Karpenter + Argo CD) | per-job engine | poor cross-run (cold) | shared `/var/run/buildkit` volume per node | scales to zero (~80% cost cut reported); cold cache  |

Upstream's on-demand pattern dedicates nodes per Dagger *version* (taints/labels), runs a
single unconstrained engine per node accepting many runners, and lets Karpenter add/remove
nodes by queue depth. It trades cache warmth for zero idle cost.

## The load-balancer pitfall

A single logical client session opens **multiple TCP connections** to the engine
([dagger#10128](https://github.com/dagger/dagger/issues/10128)). A plain k8s `Service`
load-balances round-robin, so the connections scatter across replicas and later requests
fail with `session for X not found`. Dagger is **unusable behind plain DNS/nginx/haproxy
LB** without sticky sessions. Avoid by addressing a **specific pod**
(`kube-pod://<pod>`), a headless Service, a shared host socket, or session pinning — never
a round-robin Service VIP.

## Sources

- [Kubernetes deployment](https://docs.dagger.io/reference/deployment/kubernetes/)
- [Custom / remote runner](https://docs.dagger.io/reference/configuration/custom-runner/)
- [Engine & execution model (DeepWiki)](https://deepwiki.com/dagger/dagger/3-engine-and-execution)
- [Multi-connection LB issue #10128](https://github.com/dagger/dagger/issues/10128)
- [On-demand engines: Argo CD + EKS + Karpenter](https://dagger.io/blog/argo-cd-kubernetes/)
- [Operator manual (d7yxc)](https://github.com/dagger/dagger/blob/main/core/docs/d7yxc-operator_manual.md)
- [Releases](https://github.com/dagger/dagger/releases)
</content>
</invoke>
