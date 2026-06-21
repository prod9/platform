# Dagger Engine in-cluster: StatefulSet + headless TCP, not DaemonSet/kube-pod

- **Date:** 2026-06-21
- **PR:** manual
- **Status:** accepted

## Decision

Run the in-cluster Dagger Engine as a **StatefulSet** (hard requirement), `replicas: 1`
for now, fronted by a **headless Service**. The engine listens over **TCP**
(`--addr tcp://0.0.0.0:<port>`); platform connects by setting
`_EXPERIMENTAL_DAGGER_RUNNER_HOST=tcp://<engine>-0.<svc>.<ns>.svc:<port>` — a stable
per-ordinal DNS name, never a Service VIP. Cache lives on a per-ordinal PVC
(`volumeClaimTemplates`, mounted at `/var/lib/dagger`). Traffic is **plaintext**; this is
accepted because it never leaves the cluster, and no encryption scheme (mTLS, mesh) will
be added solely for the engine link. This is the CUE-authored baseline bit deferred as
decision #3 of the [D3b-4 design-prep note](../notes/2026-06-19-d3b4-baseline-design-prep.md).

## Rationale

**Why not DaemonSet (upstream's documented default).** Dagger recommends a DaemonSet so
each CI runner reaches a *co-located* engine over a local socket and reuses per-node cache.
That model assumes the *client* is an arbitrary runner pinned to a node. Our client is
platform itself — we own `dagger.Connect` (`builder/session.go`) and choose which engine a
session talks to. A single engine already multiplexes many concurrent sessions (BuildKit
per-client sessions, cross-session content-addressed cache dedup; see
[`../reference/dagger-engine.md`](../reference/dagger-engine.md)), so a DaemonSet buys no
concurrency — only an engine per node whether used or not. We don't need that.

**Why StatefulSet specifically.** It is the only built-in workload that binds a *stable
per-replica PVC* (`volumeClaimTemplates`) and a *stable network identity* (ordinal DNS via
the mandatory headless Service). The PVC keeps the build cache warm across reschedule; the
stable DNS is what makes the connection scheme below safe. A Deployment gives neither
cleanly. Hence "hard requirement", not a default.

**Why headless + per-ordinal DNS, and why this dodges the LB bug.** A single Dagger client
session opens *multiple* TCP connections ([dagger#10128](https://github.com/dagger/dagger/issues/10128)).
Behind a normal Service VIP, kube-proxy round-robins those connections across replicas and
later requests fail with `session not found`. A headless Service (`clusterIP: None`)
publishes pod IPs directly under stable ordinal names, so addressing `<engine>-0.<svc>…`
pins every connection to one pod — the scatter cannot happen. (At `replicas: 1` it is moot;
the scheme stays correct when we add ordinals later.)

**Why not `kube-pod://`** (the scheme Dagger's own k8s guide uses). It tunnels via the
Kubernetes API (exec-style attach), so the client needs in-cluster RBAC with `pods/exec`.
`tcp://` over headless DNS is pure networking — no kubeconfig, no exec grant, smaller blast
radius. We trade Dagger's happy path for a smaller permission surface, accepting that we
must configure the engine's TCP listen addr ourselves (default is a unix socket).

**Why not sticky sessions** (`sessionAffinity: ClientIP`) to allow `replicas > 1` behind a
normal Service. ClientIP affinity pins by *source IP*. Platform is a single client pod, so
every session shares one source IP and all would pin to **one** engine — correctness
without distribution, the opposite of the goal. It is also fragile (affinity timeout, NAT,
client-IP preservation depending on Service type / `externalTrafficPolicy`). When we
outgrow one engine we scale by **addressing ordinals directly and sharding client-side in
platform** (deterministic, ours to control), or move to on-demand engines — never via
Service-LB stickiness.

**Why plaintext is fine.** Dagger's `tcp://` runner host does not encrypt the wire. The
link is platform→engine, both in-cluster; the threat model does not justify standing up
mTLS or a mesh for this one hop. Revisit only if the engine link ever crosses a trust
boundary.

## Consequences

- A CUE app authors: the StatefulSet (1 replica, `--addr tcp://`, privileged +
  `CAP_SYS_ADMIN`, `overlayfs`, `/var/lib/dagger` from `volumeClaimTemplates`), the
  headless Service, and the namespace/RBAC it needs. Rendered via the `.cue` route of
  `ops render` (it is *ours*, not a `.platform` upstream pull — decision #3).
- Platform's build path sets `_EXPERIMENTAL_DAGGER_RUNNER_HOST` to the ordinal DNS when
  running in-cluster; the bare `dagger.Connect` (auto-provision) stays the local-dev path.
  The env var is upstream-flagged experimental — re-verify on engine upgrades.
- Open follow-ups, not blocking this decision: engine cache GC / eviction config (the
  operator manual is silent on `engine.toml` knobs) and the cluster PodSecurity admission
  stance for a privileged engine pod.
</content>
