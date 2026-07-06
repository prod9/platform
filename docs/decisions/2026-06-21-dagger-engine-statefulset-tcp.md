# Dagger Engine in-cluster: StatefulSet + headless TCP, not DaemonSet/kube-pod

- **Date:** 2026-06-21
- **PR:** manual
- **Status:** accepted (revised 2026-06-21 — `replicas: 2` + round-robin dispatcher, was
  `replicas: 1` with the coordinator deferred)

## Decision

Run the in-cluster Dagger Engine as a **StatefulSet** (hard requirement), **`replicas: 2`**,
fronted by a **headless Service**. The engine listens over **TCP** (`--addr tcp://0.0.0.0:1234`,
the BuildKit-conventional port, hardcoded — no config knob). Platform connects **programmatically**
per session via `dagger.WithRunnerHost("tcp://<ip>:1234")` (a `ClientOpt`, `client.go:97`) —
**not** the `_EXPERIMENTAL_DAGGER_RUNNER_HOST` env var, which would race across concurrent
sessions. A **dumb round-robin dispatcher** in platform discovers live engine pods by
resolving the headless Service A-records (DNS, no k8s API / RBAC) and assigns each build job
an engine by `idx % n` — fixed for that job's lifetime so all its connections hit one pod.
The replica count is **not configurable** via `[ops.vars]`; an operator edits their own copy
of the CUE and platform auto-detects the new pod count from DNS. Cache lives on a per-ordinal
PVC (`volumeClaimTemplates`, mounted at `/var/lib/dagger`). Traffic is **plaintext**; accepted
because it never leaves the cluster, and no encryption scheme (mTLS, mesh) will be added solely
for the engine link. This is the CUE-authored baseline bit deferred as decision #3 of the
[D3b-4 design-prep note](../notes/2026-06-19-d3b4-baseline-design-prep.md).

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
publishes pod IPs directly under stable ordinal names, so addressing a *specific pod IP*
pins every connection to one pod — the scatter cannot happen. The dispatcher resolves the
Service A-records and picks one IP per job; round-robin is at *job* granularity, never
per-connection, so a job never spans two engines.

**Why `replicas: 2` and a dispatcher day-one, not deferred.** Multi-engine is a *capacity*
concern, not a sync one — engines are independent BuildKit daemons with their own PVC; there
is nothing to synchronize between them. The coordination ("which job, which engine, recorded
where") is platform's job, and shipping it now is cheap: `WithRunnerHost` makes per-session
engine selection a one-liner, and DNS discovery makes the pod count auto-detected. Shipping
`replicas: 1` with the dispatcher punted would mean the multi-engine path stays untested until
later — exactly the after-thought we want to avoid. `replicas: 2` is the minimum that actually
exercises round-robin. k8s spreads the pods across nodes (a preferred `podAntiAffinity` on the
engine; tolerations/node-selection strapped on orthogonally). *Consequence:* round-robin
fragments cache across the two PVCs (a module may build on `engine-0` one run, `engine-1` the
next). Accepted as the "dumb" tradeoff; a `hash(module) % n` swap later buys affinity with the
same code.

**Why not `kube-pod://`** (the scheme Dagger's own k8s guide uses). It tunnels via the
Kubernetes API (exec-style attach), so the client needs in-cluster RBAC with `pods/exec`.
`tcp://` over headless DNS is pure networking — no kubeconfig, no exec grant, smaller blast
radius. We trade Dagger's happy path for a smaller permission surface, accepting that we
must configure the engine's TCP listen addr ourselves (default is a unix socket).

**Why not sticky sessions** (`sessionAffinity: ClientIP`) to allow `replicas > 1` behind a
normal Service. ClientIP affinity pins by *source IP*. Platform is a single client pod, so
every session shares one source IP and all would pin to **one** engine — correctness
without distribution, the opposite of the goal. It is also fragile (affinity timeout, NAT,
client-IP preservation depending on Service type / `externalTrafficPolicy`). We distribute
instead by **resolving pod IPs from the headless Service and round-robining client-side in
platform** (deterministic, ours to control) — never via Service-LB stickiness.

**Why plaintext is fine.** Dagger's `tcp://` runner host does not encrypt the wire. The
link is platform→engine, both in-cluster; the threat model does not justify standing up
mTLS or a mesh for this one hop. Revisit only if the engine link ever crosses a trust
boundary.

## Consequences

- A CUE app (`apps/dagger-engine.cue`, `package apps`) on `defs.#StatefulSet` authors the
  StatefulSet (`replicas: 2`, `--addr tcp://0.0.0.0:1234`, privileged + `CAP_SYS_ADMIN`,
  `overlayfs`, `/var/lib/dagger` from `volumeClaimTemplates`, preferred `podAntiAffinity`),
  the headless Service, and the namespace/RBAC it needs. Engine image tracks the linked
  `dagger.io/dagger` SDK version (derived from the dependency at init, not a hardcoded literal).
  The engine-specific bits land as **mixins in `infra-defs`** (its
  documented mixin design); rendered via the `.cue` route of `ops render`.
- Platform's build path selects an engine per job via `dagger.WithRunnerHost`; a round-robin
  dispatcher in `builder` resolves the headless Service A-records (DNS, no RBAC) into a client
  pool and hands job `idx` the client for `idx % n`. The bare `dagger.Connect` (auto-provision)
  stays the local-dev path and the cold-start path. `WithRunnerHost` is stable SDK API;
  `_EXPERIMENTAL_DAGGER_RUNNER_HOST` (the env equivalent) is avoided — it can't be set safely
  per concurrent session.
- **Dogfood:** platform is itself one of the rendered `apps/*` and is built/published/delivered
  by its own pipeline through this engine pool. Cold-start has no unbreakable cycle — the engine
  is delivered as plain manifests (Flux/ops, not built *by* the engine), and the first platform
  image is built by a local auto-provisioned engine and pushed; thereafter in-cluster platform
  rebuilds itself via the pool. E3's verification is this dogfood build.
- Open follow-ups, not blocking this decision: engine cache GC / eviction config (the
  operator manual is silent on `engine.toml` knobs) and the cluster PodSecurity admission
  stance for a privileged engine pod.
</content>
