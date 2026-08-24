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

- [`architecture.md`](architecture.md) — the engine-owned source-to-image pipeline and the
  object model: `BuildUnit`, package layout, data-vs-behavior rules.
- [`execution-modes.md`](execution-modes.md) — the boundary between local CLI and server
  CI/CD execution, shared build/publish capabilities, app-vs-infra publish cadence, and
  this repository's own release runbook.
- [`frameworks.md`](frameworks.md) — the framework catalog and order-sensitive discovery,
  the seven-method `Framework` contract, `Step`/`Plan`/`Execute`, layouts, runtime-shape
  families, the Wolfi base, Node/pnpm provisioning, and the Go test-in-build gate.
- [`engine.md`](engine.md) — the source-to-image build facade: repository preparation,
  config interpretation, runner placement, `Session` + `Run`, the complete `Observer`
  lifecycle, publishing, registry credentials, and arch targets.
- [`releases.md`](releases.md) — release strategies (semver/datestamp/timestamp/rolling),
  `Generate` vs `Create`, tag-history recovery, and release⊥publish orthogonality.
- [`scaffolding.md`](scaffolding.md) — `platform init`: the `framework/scaffold`
  mechanism, the `Infra` framework's unconditional baseline (destination-encoded files,
  `[vars]` merge), and `scaffolding/` orchestration behind `cmd/init_cmd.go`.
- [`manifest-patch-dsl.md`](manifest-patch-dsl.md) — the line-oriented DSL for adapting
  foreign Kubernetes manifests: verbs, path grammar, `\(var)` interpolation.
- [`testing.md`](testing.md) — the two suites (`go test` / `./test.sh`), the smoke
  drift-detector contract and its golden, the per-test timeout.
- [`config-allocation.md`](config-allocation.md) — one owner per config kind across
  `platform.toml` / `infra/` / `tf/` / OCI / Flux; the no-overlap map.
- [`platform.md`](platform.md) — the platformv2 vision: an in-cluster build + delivery
  control plane (components, identity, phases, anchors).
- [`platform-server.md`](platform-server.md) — the `srv` + worker CI/CD server: GitHub-App
  auth, push/manual build triggers, engine request orchestration, and the settled
  operations table. Route surface +
  install/boot flow settled, as is the event-sourced build lifecycle; the cluster view
  (k8s + Flux state) is held for a design pass.
- [`webui.md`](webui.md) — the product front end: navigation, the route map, the repos /
  builds / engines / settings pages, and the shared components; the install wizard's UI
  stays in `installation.md`.
- [`installation.md`](installation.md) — the server install model: the installer fragment,
  the `GET /api/install` state surface, boot composition, the org-owner first-install
  gate, the install settings, by-hand App creation, and the org-wide GitHub→Flux delivery
  webhook.

Keep this list in sync when adding or removing a spec.
