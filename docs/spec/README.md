# Spec & architecture

**Current-understanding durable artifacts** — the design of the project and how
it actually fits together: design specs, RFCs, interface contracts, and
architecture / "how it works" overviews. Prose you read to *understand the
system*. Updated in place as understanding evolves; always reflects present
design, not history.

If it's a ruling on a question, that's a decision — `../decisions/`. If it's
enumerable lookup detail (every flag, every config key, a schema table), that's
`../vendor/`. If it's research, exploration, or a draft, `../scratch/`.

## Format

One file per subject: `<slug>.md` (no date prefix — describes a thing, not the
moment it was written). Add a status header (`draft`, `accepted`, `implemented`)
so readers can tell whether it still describes current design. A spec that gets
superseded moves to `../scratch/` — `spec/` holds current design only, never history.

## Index

- [`architecture.md`](architecture.md) — the build pipeline (parse → interpret → engine)
  and the object model: `BuildAttempt`/`BuildUnit`, package layout, data-vs-behavior rules.
- [`frameworks.md`](frameworks.md) — the framework catalog and order-sensitive discovery,
  the `Framework` interface (`Discover`/`Scaffold`/`Build`), layouts, runtime-shape
  families, the Wolfi
  base, and the Go test-in-build gate.
- [`engine.md`](engine.md) — the Dagger execution layer: the `Engine` client pool over
  discovered runners, `Multiplexer` fan-out, `Build`/`Publish`/`BuildAndPublish`, registry
  creds, arch targets.
- [`releases.md`](releases.md) — release strategies (semver/datestamp/timestamp/latest),
  `Generate` vs `Create`, tag-history recovery, and release⊥publish orthogonality.
- [`scaffolding.md`](scaffolding.md) — `platform init`: the `framework/scaffold` mechanism,
  the `Infra` framework's unconditional baseline (destination-encoded files, `[ops.vars]`
  merge), and `cmd/init` orchestration.
- [`manifest-patch-dsl.md`](manifest-patch-dsl.md) — the line-oriented DSL for adapting
  foreign Kubernetes manifests: verbs, path grammar, `\(var)` interpolation.
- [`config-allocation.md`](config-allocation.md) — one owner per config kind across
  `platform.toml` / `infra/` / `tf/` / OCI / Flux; the no-overlap map.
- [`platform.md`](platform.md) — the platformv2 vision: an in-cluster build + delivery control
  plane (components, identity, phases, anchors).
- [`platform-server.md`](platform-server.md) — **intended/not-yet-built:** the `srv/` tag-watch
  server, GitHub-App auth, zero platform RBAC, the `/var/cache` worktree layout.

Keep this list in sync when adding or removing a spec.
