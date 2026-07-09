# Plan: Project-Centric Builder Refactor

**Status**: Pending execution
**Law**: NO COMMITS until final all-encompassing review and landing (Session 6).

## Design Goal
Promote the `Builder` from a build-tool to a **Project Definition**. The `Builder` becomes the sole source of truth for what a project type (Go, PNPM, Infra, etc.) **is**, owning its discovery, its scaffolding requirements, and its build pipeline.

### The New Contract
The `Builder` interface expands to include:
- `Discover(wd string) bool`: Owns the logic for identifying itself (e.g., `go.mod` check, `infra` glob).
- `Scaffold() ScaffoldSpec`: Returns a declarative specification of the files and initialization steps required for the project type.
- `Build() error`: The existing build implementation.

The `scaffold` package is downgraded to a **Stateless Driver** that simply applies a `ScaffoldSpec` to the filesystem, handling the mechanism of writing files and merging `[ops.vars]`.

---

## Implementation roadmap

### Session 1: The New Contract
- Define `builder.ScaffoldSpec`.
- Expand `Builder` interface (`Discover`, `Scaffold`).
- Implement package-level `builder.Discover(wd string)` helper.
- [x] Update `scaffold` driver to handle single-builder discovery.

### Session 2: App Builder Migration
- Migrate `GoBasic`, `PNPMBasic`, and `Dockerfile` builders to implement `Discover()` and `Scaffold()`.
- Move default `[ops.vars]` from `scaffold` package into `Builder.Scaffold()`.
- Verify with `./test.sh` (drift detection on `platform.toml`).

### Session 3: First-Class Infra Builder
- Implement `InfraBuilder`:
    - `Discover()`: directory name glob check.
    - `Scaffold()`: baseline file provision.
    - `Build()`: `render` $\rightarrow$ `pack` pipeline.
- Consolidate `baseline` logic.

### Session 4: Generic Scaffold Driver
- Remove `scaffold.Analyze` and `scaffold.AnalyzeInit`.
- Transform `scaffold` into a stateless driver taking `ScaffoldSpec` + `Ops`.

### Session 5: CLI Integration
- Rewire `cmd/init.go` and `cmd/build.go` to use `builder.Discover()`.
- Full `./test.sh` smoke verification.

### Session 6: All-Encompassing Review & Landing
- Final structural audit.
- Batch commit all changes in logical slices.

---

## Verification
- **TDD**: `platform init` in clean dirs for Go, PNPM, and Infra testbeds.
- **Regressions**: Ensure `[ops.vars]` merging is preserved.
- **Smoke**: `./test.sh` must result in `UNCHANGED`.
