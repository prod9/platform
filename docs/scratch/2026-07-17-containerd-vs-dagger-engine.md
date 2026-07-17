<!-- not spec/decision because: exploration of a possible future direction; no ruling made —
the engine stays Dagger unless/until this graduates to an ADR -->

# containerd vs the Dagger engine — could we drive containerd/BuildKit directly?

Produced by a web-research subagent (Haiku, 2026-07-17); citations inline. Figures from
third-party blogs (the mW idle-power numbers especially) are reported as found, not
independently verified.

## Summary verdict

**Feasible but costly.** Replacing Dagger with containerd + BuildKit is technically
viable. We'd forfeit Dagger's semantic operation-level DAG caching, typed SDK, and
lifecycle orchestration; we'd gain transparency, a lighter macOS footprint, and fewer
layers. The estimate for hand-rolled replacement infrastructure is **4–8 weeks**. The
pattern is established — Earthly, Depot, and Okteto all drive BuildKit directly — but
every one of them hand-rolled infrastructure *around* BuildKit, and none replaced
BuildKit itself. containerd alone cannot do it: it has no build DAG, no solver, no
Dockerfile/LLB execution — builds require BuildKit regardless.

## 1. What the Dagger engine actually is

A containerized BuildKit daemon wrapped in a GraphQL API layer. Dagger adds on top of
BuildKit:

- **GraphQL API server** — per-session endpoint; every SDK call is a GraphQL query.
- **Typed SDKs** — codegen clients; method chaining composes the DAG imperatively.
- **LLB translation** — `WithExec`/file ops compile to BuildKit's LLB protobuf DAG.
- **Semantic caching** — cache keys at the *operation* level (receiver id + field +
  args, recursively hashed), finer than Dockerfile layer caching.
- **Secrets** — first-class `Secret` type, never lands in logs/caches/filesystems.
- **Ephemeral services** — canonical hostnames, auto-tunnel to localhost, health
  checks (what `preview` rides).

Sources: [architecture overview](https://deepwiki.com/dagger/dagger/1.2-architecture-overview),
[engine and execution](https://deepwiki.com/dagger/dagger/3-engine-and-execution),
[API reference](https://docs.dagger.io/api/reference/).

## 2. What containerd provides — and doesn't

Provides: image pull/push, snapshotters, task lifecycle (create/start/stop/kill/wait),
exec into tasks, namespacing, metrics.

Does **not** provide: any build DAG, LLB solver, Dockerfile execution, build-layer
caching, port publishing (networking is CNI's job — containerd creates empty netns),
or credential helpers (static auth.json/hosts.toml only — no osxkeychain).

Sources: [containerd](https://github.com/containerd/containerd),
[hosts docs](https://containerd.io/docs/main/hosts/),
[iximiuz on containerd networking](https://labs.iximiuz.com/challenges/access-containerd-container-with-no-published-ports).

## 3. Driving BuildKit from Go

`github.com/moby/buildkit` is the canonical, production-grade Go client (Docker/Moby
use it themselves). LLB construction in Go is mature: `llb.Image`, `llb.Local`,
`state.Run(llb.Shlex(…))`, `llb.AddMount(…, llb.AsCacheMount("golang"))` — cache
mounts with sharing modes are fully supported. Registry cache export (inline/min/max)
is a BuildKit capability Dagger does *not* currently expose — a point in favor of
going direct if cross-run CI cache matters.

Dagger SDK vs raw LLB: high-level chaining + services + secrets vs fine-grained
control and no daemon/API layer. Platform is Go-only, so Dagger's multi-language SDK
surface buys us nothing.

Sources: [buildkit Go pkg](https://pkg.go.dev/github.com/moby/buildkit),
[client/llb](https://pkg.go.dev/github.com/moby/buildkit/client/llb),
[examples](https://github.com/moby/buildkit/tree/master/examples).

## 4. exec / preview / services without Dagger

- **nerdctl** — Docker-compatible containerd CLI: `run -it`, `-p` port mapping via
  CNI, rootless mode. Closest parity for `exec`-style use.
- **Raw containerd** — task exec works (`ctr task exec`); port publishing is fully
  manual CNI wiring.
- **BuildKit gateway exec** — build-step-scoped, ephemeral; not a services runtime.

Dagger's tunnel/health-check/service-DAG abstraction is exactly the part with no
off-the-shelf replacement — we'd hand-roll nerdctl/CNI glue (~1000 lines + shell) for
what `preview` does today.

Sources: [nerdctl](https://github.com/containerd/nerdctl),
[dagger services](https://docs.dagger.io/features/services/),
[skaffold port-forwarding](https://skaffold.dev/docs/port-forwarding/).

## 5. Publish auth (ghcr + osxkeychain)

- **BuildKit**: works with docker credential helpers — buildctl (client side) reads
  `~/.docker/config.json` `credsStore: osxkeychain`, same as our `publish` today.
- **containerd**: no credential-helper support; static auth.json workaround only,
  re-primed by `docker login` when tokens rotate.

Sources: [BuildKit configure](https://docs.docker.com/build/buildkit/configure/),
[containers-auth.json](https://github.com/containers/image/blob/main/docs/containers-auth.json.5.md).

## 6. macOS story

containerd/BuildKit need a Linux VM either way. Options: Colima (MIT, brew-installed,
lightest), Lima (manual, flexible), Docker Desktop (heaviest, paid >250 employees),
OrbStack (fastest startup, free personal / $8mo pro). Reported figures: Colima ~180mW
idle vs Docker Desktop ~726mW; 70–95% native FS throughput vs 25–40% under Docker
Desktop (third-party blog numbers — treat as directional). Apple's native container
runtime (macOS 26) is experimental and currently broken for this use.

Sources: [colima](https://github.com/abiosoft/colima), [lima](https://github.com/lima-vm/lima),
[orbstack](https://orbstack.dev/),
[colima vs docker desktop](https://oneuptime.com/blog/post/2026-02-08-how-to-choose-between-colima-and-docker-desktop-on-macos/).

## 7. Precedent — Earthly, Depot, Okteto

All three drive BuildKit directly; none replaced its solver. What they hand-rolled sits
*around* BuildKit: Earthly — Earthfile→LLB compiler + remote-runner cache satellites;
Depot — warm instance pools + Ceph-backed distributed cache (~6x cache throughput) +
native multi-arch fleets; Okteto — in-cluster BuildKit pools + content-hash build
dedup + resource-aware scheduling.

Sources: [earthly satellites](https://docs.earthly.dev/earthly-cloud/satellites),
[depot magic explained](https://depot.dev/blog/depot-magic-explained),
[buildkit in depth](https://depot.dev/blog/buildkit-in-depth),
[okteto build service](https://www.okteto.com/docs/core/build-service),
[dagger discussion #1874](https://github.com/dagger/dagger/discussions/1874).

## Capability table

| Capability            | Dagger                      | containerd+BuildKit           | containerd alone |
| --------------------- | --------------------------- | ----------------------------- | ---------------- |
| Build DAG             | yes, operation-level cache  | yes, layer-level cache        | **no**           |
| Cache mounts          | yes                         | yes                           | no               |
| Registry cache export | not exposed                 | **yes**                       | no               |
| Image pull/push       | yes                         | yes                           | yes              |
| Task exec             | yes                         | yes (nerdctl/ctr)             | yes (ctr)        |
| Port mapping/services | yes (tunnels, healthchecks) | manual (nerdctl + CNI)        | manual CNI only  |
| Typed Go SDK          | yes                         | Go client lib (LLB, no sugar) | Go client lib    |
| Multi-arch builds     | transparent                 | manual QEMU orchestration     | no               |
| Credential helpers    | via docker creds            | BuildKit yes / containerd no  | no               |
| Secrets               | first-class, memory-only    | env/mounts                    | mounts           |
| macOS                 | docker daemon               | VM (colima/lima)              | VM               |

## Build-vs-buy delta (what we'd hand-roll)

| Piece                                              | Effort    |
| -------------------------------------------------- | --------- |
| DAG composition wrapper over LLB                   | 1–2 weeks |
| buildkitd lifecycle (start/socket/shutdown)        | 3–5 days  |
| Networking + preview tunneling (nerdctl/CNI glue)  | 1–2 weeks |
| Cache orchestration beyond layer cache (optional)  | 1–2 weeks |
| Registry auth/secrets plumbing                     | 3–5 days  |
| Multi-module orchestration                         | ~1 week   |
| Typed chaining SDK (optional, Go-only need)        | 2–3 weeks |

Critical path ≈ 4–5 weeks; full scope 4–8 weeks. Not hand-rolled under any plan: the
LLB solver, the runtime, image push/pull — those stay BuildKit/containerd.

## Agent's recommendation (unreviewed)

Stay on Dagger unless (a) the VM/daemon footprint becomes a hard constraint, (b) our
iteration pattern doesn't benefit from operation-level caching, or (c) the 4–8-week
infra investment is acceptable. If migrating: Colima VM, start with raw
BuildKit/buildctl for an MVP, layer the LLB wrapper incrementally, keep BuildKit's
native caching until profiling says otherwise.
