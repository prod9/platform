# Engine

Status: **implemented.** The `engine/` package — the Dagger execution layer that runs a
`BuildAttempt`'s units and pushes their images. Sits at the tail of the pipeline
([`architecture.md`](architecture.md)): `BuildAttempt ─▶ engine.Build ─▶ images`.

## What the Engine is

`Engine` is a process-wide handle to a fleet of Dagger runners — shaped like `sql.DB`: a
concurrency-safe connection pool, dialed lazily and reused, built once from config and
shared across the process. It orchestrates two single-purpose units:

- **runners** ([`runners.go`](../../engine/runners.go)) — which endpoints exist.
- **clients** ([`clients.go`](../../engine/clients.go)) — one reused, ping-checked
  `*dagger.Client` per endpoint.

`Engine` itself holds no lock — only an atomic round-robin `cursor`. The happy path is
discover → pick → get.

### Lifecycle

| Call                       | Role                                                              |
| -------------------------- | ---------------------------------------------------------------- |
| `New(cfg)`                 | build the handle from an fx config source; dials nothing yet     |
| `NewContext(ctx, eng)`     | carry `eng` on a context so `Build`/`Publish` resolve it         |
| `FromContext(ctx)`         | Must-style fetch; panics if absent (engine is a precondition)    |
| `LookupFromContext(ctx)`   | comma-ok fetch                                                    |
| `Client(ctx)`              | next endpoint round-robin → a live client (ad-hoc: `ls`/preview) |
| `Clean(ctx)`               | prune every fleet engine's local cache (drives `platform clean`) |
| `Close()`                  | tear down every dialed connection; call once at shutdown         |

Commands open one engine, defer `Close`, and stash it on the context via `NewContext`;
downstream `Build`/`Publish`/`BuildAndPublish` pull it back with `FromContext`. Mirrors
fx/data's request-scoped `*sqlx.DB`.

## Runner discovery

`runners` resolves the configured Dagger endpoints via DNS — no k8s API, no RBAC:

| Config               | Default | Meaning                                                     |
| -------------------- | ------- | ----------------------------------------------------------- |
| `DAGGER_ENGINE`      | unset   | headless-Service DNS of the engine pool                     |
| `DAGGER_ENGINE_PORT` | `1234`  | engine pod port (mirrors `apps/dagger-engine.cue`)          |

`Hosts(ctx)` looks up the DNS name and returns one `tcp://<addr>:<port>` per resolved pod,
sorted for stable round-robin. It reports **only what it finds**:

- `DAGGER_ENGINE` unset → empty slice, no lookup.
- DNS resolves to nothing → empty slice.
- lookup failure → a real error, surfaced.

Falling back to a local engine is **not** the runner's decision — it reports emptiness and
the core decides. `Engine.resolveHosts` maps an empty result to a single empty-string
host, and `dialEngine` reads an empty host as "let Dagger auto-provision and reuse the
local engine." So unset `DAGGER_ENGINE` is an explicit operator choice for local, never
inferred.

The resolver caches per the DNS record TTL, so a new pod becomes selectable as soon as DNS
reflects it — no restart.

### Client pool

`clients` caches one `*dagger.Client` per host. `Get` validates a cached client with a
cheap `Version()` ping and redials when the engine has gone (graceful DNS removal or a
crash), so callers always receive a live client — no separate prune step, nothing closed
mid-build. The lock is held only around map reads/writes, never across a dial or ping;
concurrent dial races keep one winner and close the loser. Liveness during a run is the
ping's job; `Close` only runs at shutdown.

## Fan-out

`Build` and `Publish` fan out over the attempt's units via
[the in-package `multiplexer`](../../engine/multiplexer.go) — a generic `[TIn, TOut]` worker
that spawns one goroutine per input, collects results index-aligned under a mutex, and
`WaitGroup`-joins. One unit → one goroutine.

Each goroutine calls `Engine.Client(ctx)`, which round-robins the cursor over the
currently-discovered hosts — so units spread across the fleet. `Build` wraps each unit in
`context.WithTimeout(ctx, unit.Timeout)`, invokes `unit.Framework.Build`, then `Sync`s the
container to force the work eagerly. A per-unit failure is captured on that unit's
`BuildResult.Err`, never aborting its siblings.

`BuildResult` carries the `*dagger.Client` that built its container. The container is
bound to the engine that produced it, so `Publish` — and any caller that keeps operating
on the container (preview's tunnel, via `BuildResult.Client()`) — must reuse that same
client.

## Build vs Publish vs BuildAndPublish

Three entrypoints, all reading the engine off the context:

- **`Build(ctx, attempt)`** — runs every unit, returns `[]BuildResult`. `ErrNoJobs` on an
  empty attempt.
- **`Publish(ctx, builds...)`** — pushes every successfully-built container, reusing each
  build's own client so the registry secret is minted by the engine that owns the
  container. Skips builds already carrying an `Err`. Returns `[]PublishResult` (adds
  `ImageName` + `ImageHash`), logging each via `buildlog.Image`.
- **`BuildAndPublish(ctx, cfg, args, tag)`** — the composed unit: `AttemptFrom(cfg, args,
  PublishBuild)`, suffix each unit's `ImageName` with `:tag`, `Build`, then `Publish`,
  returning the `[]PublishResult` (so a driver can record what shipped) plus any
  per-result errors joined. Reuses the caller's engine rather than opening its own.

### One publish engine, two drivers

`BuildAndPublish` is the reusable build+tag+push unit — deliberately in `engine`, not
trapped in a `cmd/` file — so two front-ends embed the *same* logic:

- **local CLI `publish`** — [`cmd/publish.go`](../../cmd/publish.go) resolves the
  release name, opens an engine, and calls `BuildAndPublish`. You stand in for the CI
  server.
- **tag-watch platform server** — [`srv/builds/runner.go`](../../srv/builds/runner.go)'s claim loop
  invokes the same unit on each queued tag build and records the returned
  `[]PublishResult`. The trigger lives only in the server; the CLI never watches.

`release` (cut a tag) and `publish` (build + push) are orthogonal — neither implies the
other, and there is no `deploy` verb. See
[delivery-verbs-are-orthogonal](../decisions/2026-07-05-delivery-verbs-are-orthogonal.md)
for the full rationale.

## Registry credentials

`Publish` reads three fx env-config values off the engine's config source:

| Config              | Role                          |
| ------------------- | ----------------------------- |
| `REGISTRY`          | registry host for auth        |
| `REGISTRY_USERNAME` | registry user                 |
| `REGISTRY_PASSWORD` | registry secret (set via `client.SetSecret`, never inlined) |

When `REGISTRY_USERNAME` is empty, `Publish` skips `WithRegistryAuth` entirely — Dagger
then pushes with the **local docker credentials** (osxkeychain). That is the local-publish
path: a `platform publish` on a laptop needs no `REGISTRY_USERNAME`/`PASSWORD`, only a
docker login to ghcr. The env creds are for a server driver with no local docker config.

## Arch targets

The unit's `Arch` is resolved at interpret time from the attempt's `Purpose`
([`framework/unit.go`](../../framework/unit.go)) — the engine reads the field, never a
call argument:

| Purpose        | Arch source    | Default             |
| -------------- | -------------- | ------------------- |
| `LocalBuild`   | `local_arch`   | `auto` = host arch  |
| `PublishBuild` | `publish_arch` | `amd64`             |

`build`/`preview`/`export`/`ls` run `LocalBuild` for fast native iteration; `publish` runs
`PublishBuild` so an arm laptop never ships an unrunnable image. Bare archs become
`linux/<arch>`; `auto` tracks `runtime.GOARCH`. The infra `FROM scratch` manifest image
carries no executable, so arch is irrelevant to it.
