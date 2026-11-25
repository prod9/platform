# Spec & architecture

**Current-understanding durable artifacts** — the design of the project and how
it actually fits together: design specs, RFCs, interface contracts, and
architecture / "how it works" overviews. Prose you read to *understand the
system*. Updated in place as understanding evolves; always reflects present
design, not history.

If it's a ruling on a question, that's a decision — `../decisions/`. Our own flags,
config keys, API routes, and schemas belong here; third-party lookup detail belongs in
`../vendor/`. Unsettled exploration belongs in `../scratch/`.

## Format

One file per subject: `<slug>.md` (no date prefix — describes a thing, not the
moment it was written). Add a status header (`draft`, `accepted`, `implemented`)
so readers can tell whether it still describes current design. A spec that gets
superseded moves to `../scratch/` — `spec/` holds current design only, never history.

## Index

- [`architecture.md`](architecture.md) — the build pipeline driven by engine and the
  object model: `BuildUnit`, package layout, data-vs-behavior rules, the command surface.
- [`execution-modes.md`](execution-modes.md) — the boundary between shared build/publish
  capabilities, the CLI that invokes them, the app-vs-infra delivery convention, and this
  repository's own release runbook.
- [`lua-render.md`](lua-render.md) — `render`: `apps/*.lua` on the sandboxed Lua VM, the
  rock fetch and cache, `platform.vars`, schema validation, and the templates shipped in
  the rock.
- [`frameworks.md`](frameworks.md) — the framework catalog and order-sensitive discovery,
  the seven-method `Framework` contract, `Step`/`Plan`/`Execute`, layouts, runtime-shape
  families, the Wolfi base, Node/pnpm provisioning, and the Go test-in-build gate.
- [`engine.md`](engine.md) — the reusable build driver: runner placement, `Session` +
  `Run`, the complete `Observer` lifecycle, publishing, registry credentials, and arch
  targets.
- [`releases.md`](releases.md) — release strategies (semver/datestamp/timestamp/rolling),
  `Generate` vs `Create`, tag-history recovery, and release⊥publish orthogonality.
- [`scaffolding.md`](scaffolding.md) — `platform init`: the `framework/scaffold`
  mechanism, the `Infra` framework's baseline from the `infra-lua` templates, the
  `[vars]` merge, and `scaffolding/` orchestration behind `cmd/init_cmd.go`.
- [`testing.md`](testing.md) — the two suites (`go test` / `./test.sh`), the smoke
  drift-detector contract and its golden, the per-test timeout.
- [`config-allocation.md`](config-allocation.md) — one owner per config kind across
  `platform.toml` / `infra/` / `tf/` / OCI / Flux; the no-overlap map.
- [`interfaces.md`](interfaces.md) — CLI presentation rules: minimal translation,
  system-aligned terms and values, derived-value hints, and actionable errors.
- [`terminology.md`](terminology.md) — canonical product, execution, verb, artifact,
  value, and control vocabulary shared across every surface.

Removed with the v1.0 rescope, when `platform` became a CLI that renders Lua: the server
specs `platform-server.md`, `installation.md` and `webui.md`, the control-plane vision
`platform.md`, and `manifest-patch-dsl.md`. Their last versions are in git history at tag
`v0.10.0`; the rulings are the
[server-removed](../decisions/2026-11-25-server-removed-platform-is-a-cli.md) and
[DSL-removed](../decisions/2026-11-25-dsl-removed-for-lua-templates.md) decisions.

Keep this list in sync when adding or removing a spec.
