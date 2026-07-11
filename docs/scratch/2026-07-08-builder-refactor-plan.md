<!-- not spec/decision because: active implementation plan/tracker; the settled design is
promoted to spec/frameworks.md + decisions/2026-07-11; this file only tracks migration state -->

# Plan: Framework Refactor

**Status**: **IMPLEMENTED 2026-07-11 (later session)** — all seven slices + the
terminology addendum are in the tree, uncommitted, vs reference point `e3fa5c6` (~105
files). `go build`/`vet`/`test ./...` green; a fresh-eyes audit agent confirmed
correctness + design conformance and its findings were fixed (deprecation note on both
alias cases, no-op `renderTemplate` deleted, lexicon comment strays, `@vN`-strip test).
Remaining before commit: operator reviews the diff and runs `./test.sh` (docker) —
expect **more than one** CHANGED (init prompts/plan output changed shape, scaffold now
writes `framework =` + cue.mod), review and `--commit`. Known leftovers, deliberate:
`framework.All()` dead export (removal unauthorized), `Info.ImagePrefix`
prompted-but-unread, launcher template hole-less w/ stale version (all pre-existing).

## Target architecture (locked)

A **`Framework` is the sole owner of a project type** — it owns recognizing itself,
scaffolding itself, building itself. Only two things are *not* the framework: the
`platform.toml` data model, and the `init` command's human orchestration.

Packages — acyclic graph `project ← framework/scaffold ← framework ← cmd`:

- **`project/`** — the `platform.toml` model, both directions: `Generate` + the surgical
  `[ops.vars]` merge. Unchanged by this refactor; correct.
- **`framework/scaffold/`** — THE ONE files/templating mechanism: render templates with
  data, write files. Generic — no discover, no orchestration, no per-type data, no "spec".
- **`framework/`** (renamed from `builder/`) — the `Framework` interface (`Discover`,
  `Scaffold`, `Build`), the concrete frameworks, and the package-level `Discover(wd)`
  resolver. **`Infra` embeds its own baseline assets + version pins here.**
- **`cmd/init`** — orchestration: gather operator inputs → `framework.Discover` →
  `fw.Scaffold` → confirm → write.

**Dissolved packages:** `baseline/` (its templating → `framework/scaffold`; its embedded
infra files + version pins + routing → *into the `Infra` framework*) and top-level
`scaffold/` (mechanism → `framework/scaffold`; discover → `framework/`; orchestration →
`cmd/init`).

### The `Framework` contract

- `Discover(wd) bool` — the framework's own heuristic.
- `Scaffold(...)` — **rich, per-framework.** Returns the framework's full contribution: its
  `platform.toml` module, default `[ops.vars]`, the files it ships (rendered via
  `framework/scaffold`), its default strategy seed, and whether it needs a fresh git repo.
- `Build(...)` — unchanged.

**No `IsInfra()`, no app-vs-infra predicate or branch anywhere.** `Infra.Scaffold` simply
*does more* — contributes its baseline files, `strategy="latest"` seed, and create-repo
need, as data. The app/infra distinction is purely how much each `Scaffold` does
(polymorphism).

**Strategy** is a scaffold-time *default seed* the framework contributes (what a fresh
`platform.toml` gets), **never** a runtime `Framework.Strategy()` method. `"latest"` for
infra. **Registry creds** are defaulted empty placeholders in the committed baseline CUE
(operator hand-edits), never prompted. The component **picker is removed** — the full
default baseline installs unconditionally.

## Currently landed (uncommitted, vs `e3fa5c6`)

Foundation (green): `project/` owns `platform.toml` (`Generate` + relocated vars-merge);
`Scaffold(ctx,wd)` seam; `Interface`→`Framework` type rename (package still `builder/`).
A partial C/D landed **with deviations the target reverses** — treat these as work to undo:

- `IsInfra()` method added → **remove**.
- baseline install left in the scaffold driver gated by `fw.IsInfra()` → **move into
  `Infra.Scaffold`**.
- `ScaffoldSpec.Modules` is a `map` → **narrow to a single `Module`**.
- `ScaffoldSpec`/`FileSpec` in `builder/scaffold.go` → **move to `framework/scaffold/`**.
- `baseline/` still a package → **dissolve into the `Infra` framework**.
- top-level `scaffold/` still holds orchestration → **dissolve into `cmd/init`**.

## Migration slices (keep `go build` + `go test ./...` green at each boundary)

1. **One mechanism** — move files/templating into `framework/scaffold/` (from
   `builder/scaffold.go` + `baseline.Render`).
2. **Infra owns its baseline** — move the embedded assets + version pins + routing into the
   `Infra` framework; delete the `baseline/` package.
3. **Rich `Infra.Scaffold`** — it produces its baseline `Files` (holes unresolved) + `Vars`
   + strategy seed + create-repo need; wire `ScaffoldSpec.Files` live.
4. **Eliminate `IsInfra()`** — the driver writes whatever `Scaffold` returns, uniformly; no
   infra branch. Strategy seed + git-gating become framework-set spec fields.
5. **Dissolve `scaffold/`** — discover stays in `framework/`; orchestration → `cmd/init`.
6. **Rename** `builder/` → `framework/`.
7. **Cleanups** — `Modules`→single `Module`; `maps.Copy`; drop the unused `ctx`/`error` if
   still unused; validate-before-discover order.

## Addendum — terminology renames (2026-07-11 lexicon ADR)

Code legs of the
[terminology lexicon](../decisions/2026-07-11-terminology-lexicon.md). Fold each into the
slice that already touches the file; anything left over is a final slice 8. Docs already
use the new words.

- **toml key `framework`** — `[modules]` key renames `builder` → `framework`; old key read
  as deprecated alias (emit a deprecation note), scaffold writes only `framework`. (Slice 6
  seam — lands with the package rename.)
- **`FindBuilder` → `FindFramework`**, `knownBuilders` → `knownFrameworks`,
  `ErrBadBuilder`/`ErrNoBuilder` follow. (Slice 6.)
- **`BuildUnit.Builder` → `BuildUnit.Framework`** (already tracked deferred) and
  **`BuildUnit.Platform` → `BuildUnit.Arch`**. Specs then update their two code-true
  mentions.
- **Delete `Class()`** from the `Framework` contract + the `Class` type/consts — unused,
  removal authorized in-session. Taxonomy stays as prose in frameworks.md (done).
- **releases:** `NameComponent`/`Options.Component` → `Bump`/`Options.Bump` with
  `BumpAny`/`BumpPatch`/`BumpMinor`/`BumpMajor`; `Release.Render` → `Changelog`. Flags
  unchanged.
- **dsl:** internal `engine` struct → `interpreter`; internal string-"render" naming folds
  into `resolve` vocabulary.
- **baseline leg (dissolves in slices 1–3 anyway):** the `Install()`-that-installs-nothing
  name must not survive the move — the relocated function is named for what it does
  (returns file bytes). Install-time templating never takes the word "render".
- **vestigial:** delete the `deploy` mention in `project.go` comments and the `"deploy"`
  entry in default `Excludes`; drop the dead `environments` key from the repo's own
  committed `platform.toml`.

## Verification

`go test ./...` green throughout. Operator runs `./test.sh` (docker) — expect one `CHANGED`
(the `defaults-basics.cue` creds comment), re-record with `--commit`; that red is expected,
not a regression.
