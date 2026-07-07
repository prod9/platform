# Repo streamline — first-principles consolidated design (2026-07-07)

Supersedes the command-surface-only draft (`2026-07-07-ops-flatten-proposal.md`). Scope is
now the whole repo: package layout, the config model, discovery, scaffolding, and the two
publish pipelines — made to make sense end-to-end. For eyeball before any code moves.

## The core realization

platform has **one spine and three actions**. The spine is the parsed `platform.toml`
(the `project` config model). Everything platform does is one of three actions *on* a
project:

- **scaffold** it (write `platform.toml` + script, optionally seed an infra repo) — `init`
- **build → publish** its app **image** (Dagger pipeline) — `build` / `publish`
- **render → publish** its infra **manifests** (CUE/DSL pipeline) — `render` / `publish`

Today that clean shape is obscured because the codebase splits *app-side* from *infra-side*
into parallel structures — a `cmd/ops/` command namespace, a 30-line `ops/` model package,
`bootstrapper` (app scaffold) beside `baseline` (infra scaffold), `engine` (image) beside
`gitops` (manifest) — with no shared frame. The rework (Flux pull model, no `deploy`, no
environments) dissolved the reasons for most of those splits; the structure never caught up.

## First-principles domain model

Six concepts, no more:

1. **Project** — the parsed, defaulted, inferred config. The spine. A project is one of two
   *kinds*, distinguished by what it contains, not by a flag: an **app** (has `[modules]`)
   or an **infra repo** (has `apps/` + `[ops]`). The delivery target (`[ops]`) is a
   **sub-model of Project**, not its own package.
2. **Discovery** — "what stack is in this directory?" One concern: detect the builder + its
   modules. It is a **scaffold-time** concern (it populates `[modules]` at `init`); the
   build path never re-discovers — it reads `cfg.Modules`. So discovery has exactly one real
   consumer (init) and needs no standalone command.
3. **Scaffold (`init`)** — assemble a write-plan: `platform.toml` + `platform` script,
   plus optional baseline components. **App-vs-infra is decided by component selection**, not
   a mode. Pick none → app onboarding; pick components → infra repo. One plan builder.
4. **Build** — interpret `[modules]` → `BuildUnit`s → Dagger `engine` → images. (Already
   clean per `architecture.md`; untouched.)
5. **Render** — infra `apps/` (`.cue` + `.platform`) → manifest tree, via the linked CUE
   engine + `dsl`. (Untouched.)
6. **Publish** — build a module's image and push it. **Uniform** across app and infra:
   [the oras-drop decision](../decisions/2026-07-05-infra-publishes-as-plain-image-retire-oras.md)
   makes infra a *builder* whose `FROM scratch` image carries the rendered manifests, so
   `publish` is one path, not a two-producer dispatch. (My earlier "keep producers separate"
   pushback was reasoning from the oras code that decision retires — withdrawn.)

## Target package layout

| Package         | Responsibility (streamlined)                             | Change                          |
| --------------- | -------------------------------------------------------- | ------------------------------- |
| `project/`      | config spine: parse/default/infer; **includes the `Ops` sub-model** | **absorb `ops/`**       |
| `builder/`      | interpret config → build units; per-stack strategies; **discovery** | docstring discovery as scaffold-time |
| `engine/`       | Dagger runtime; image build+publish                      | rename internal `discovery`→`fleet` |
| `gitops/`       | infra render + manifest publish (uses `dsl`)             | unchanged                       |
| `dsl/`          | directive language                                       | unchanged                       |
| `baseline/`     | embedded seed files (the init data)                      | unchanged (only `init` uses it) |
| `scaffold/`     | one plan builder for `init` (was `bootstrapper`)         | **rework** (see below)          |
| `releases/`     | git tag strategies                                       | unchanged                       |
| `gitctx/`       | git context helpers                                      | unchanged                       |
| `internal/`     | buildlog, multiplexer, timeouts                          | unchanged                       |
| ~~`ops/`~~      | —                                                        | **deleted** (folded to project) |
| ~~`cmd/ops/`~~  | —                                                        | **dissolved** (verbs promoted)  |

Import graph stays one-directional: `cmd → {engine → builder → project, gitops → {dsl,
project}, scaffold → {builder, baseline, project}}`. No cycles introduced (verified: `ops/`
is imported only by `project`; `baseline`/`bootstrapper` only by the two scaffold commands;
`gitops` does not import `baseline`).

## Command surface (final)

```
init   build   configure   exec   export   ls   preview   publish   release   render   vanity
```

- `init` — scaffold (was `bootstrap` + `ops init`).
- `publish` — dispatch on repo kind: `[modules]` → image (engine); `apps/`+`[ops]` →
  manifest artifact (gitops). (was `publish` + `ops publish`).
- `render` — infra render (was `ops render`).
- **dropped:** `discover` (re-run `init` to see detection), `bootstrap` (renamed), the `ops`
  group.

## The specific folds

### 1. `ops/` → `project/` (pure move)
The whole `ops/` package is `Ops{Image,Tag,Vars}` + `Ref()` + one error — a config
sub-model. Nothing imports it but `project`; it imports nothing back. No cycle reason to
keep it separate. Move the type into `project`, delete the package. (This revises
`architecture.md`, which had reserved an `ops/` package for the delivery model — the rework
made the model small enough to live in `project`.)

### 2. `bootstrapper` → `scaffold`, one plan builder
`Analyze` (app) and `AnalyzeInit` (infra) are near-duplicates: both call `planProjectFile`
+ `renderTemplate`; infra adds cue.mod scaffold, creates git instead of requiring it, and
folds in baseline components. Collapse to **one** `Analyze` that always writes the shared
base (platform.toml + script), ensures a git repo (no-op if present), and folds in whatever
components the picker selected (cue.mod scaffolded only when a CUE/app component is chosen).
Discovery runs inside it to populate `[modules]`. `baseline` stays as the embedded-data
package `scaffold` consumes.

### 3. Discovery — one concept, one home
`builder.Discover` is the only discovery that matters to users; its real caller is scaffold
(populating modules). Give it a home there and a clear docstring; drop the `discover`
command (redundant preview). Separately, `engine/discovery.go` is an **unrelated** concept
(Dagger *endpoint* discovery) sharing the name — rename it `fleet`/`endpoints` to kill the
collision. Not a grand unified "classification" abstraction — that would over-scaffold; the
`publish` dispatch reads a cheap config signal (`len(Modules) > 0` vs `apps/` present).

### 4. `publish` — genuinely uniform (infra is a builder)
Per [infra-publishes-as-plain-image-retire-oras](../decisions/2026-07-05-infra-publishes-as-plain-image-retire-oras.md),
infra is a builder module whose `Build` renders `apps/` and packs the manifest tree into a
`FROM scratch` image; Flux consumes it via `layerSelector`. So both publishes ARE the same
work at the mechanism level — Dagger builds an image, Dagger pushes it with local-docker
creds. `publish` needs **no repo-kind dispatch**: it builds+publishes the configured modules,
and infra is just one such module. The oras packer + Flux media types + `gitops.Publish` +
`gitops/registry.go` retire; `gitops` keeps only `Render`. This also erases the env-cred
asymmetry (osxkeychain vs `REGISTRY_USERNAME/PASSWORD`) and drops `oras-go` from `go.mod`.

("ops publish and publish is the same work anyway" was right; my earlier verb-not-producers
split was wrong — it read the current oras code instead of this decision. Withdrawn.)

## Also-found cleanups (flag, fold in opportunistically)

- `engine/run.go:20` — comment still names the removed `deploy` command. Stale.
- `NormalizeVars` lives in `project` but only `gitops` consumes it — move to `gitops`.
- `PLATFORM` env override sets *both* `LocalArch` and `PublishArch` from one value — a legacy
  knob that conflates the two arches the rest of the model carefully split. Candidate for
  removal or a documented compat note; not in the critical path.

## Docs / specs to update

`architecture.md` (design-of-record: drop the `ops/` package row, fold discovery + the
two-producer publish into the pipeline description), `config-allocation.md`, the
[delivery-verbs ADR](../decisions/2026-07-05-delivery-verbs-are-orthogonal.md) (amend the
"infra config is its own concern" line — verb unifies, producers don't),
`manifest-patch-dsl.md`, `migration.md`, `PLANS.md`, `CLAUDE.md`, `tests.cue`
(`ops render`→`render`, drop `discover`), smoke golden re-record.

## Migration slices (each lands green on its own)

1. **Fold `ops/` → `project/`.** Pure type move; no behavior change. Smoke untouched.
2. **Rework `bootstrapper` → `scaffold`** (one `Analyze`), both `bootstrap`/`ops init`
   commands still wired to it. Behavior-preserving; unit tests move.
3. **Infra-as-builder + retire oras** — add the `platform/infra` builder (`Build` renders
   `apps/` → `FROM scratch` image); delete `gitops.Publish` + `gitops/registry.go` + `oras-go`
   from `go.mod`; `gitops` keeps `Render`. Verify the stage9 `OCIRepository` `layerSelector`
   consumes the plain image. (Per the oras-drop decision.)
4. **Dissolve `cmd/ops/`**: promote `init`/`render`/`publish` (now uniform), drop `discover`.
   Command-surface change; smoke re-record.
5. **Rename `engine` discovery→`fleet`**; stale-comment (`engine/run.go:20`) + `NormalizeVars`
   cleanups.
6. **Docs/specs sweep.**

## Decisions locked (chakrit, this session)

- **App-vs-infra is detected by the repo name**, not the picker: a repo **named `infra`**
  (`filepath.Base(wd) == "infra"`) is an infra repo → full GitOps baseline; anything else is
  an app repo → just `platform.toml` + build script. (Supersedes the earlier picker-decides
  framing in §Fold mechanics above.)
- The `init` command is aliased **`scaffold`**.
- `engine`'s runner discovery is renamed **`pool`** was rejected (collides with the existing
  `Pool`/`clients`); use the Dagger term — the unit is renamed to reflect the **runners** it
  discovers.
- Package renamed `bootstrapper/` → `scaffold/`; `baseline/` stays its own package.

## Progress

- ✅ Slice 0 (spec), 1 (`ops`→`project`), 2 (`scaffold` rename + `init`/`scaffold` alias) —
  landed and committed, green.
- ▶ Remaining: command merge (`bootstrap`+`ops init`→top-level `init`, drop `discover`);
  infra-as-builder + retire oras; `engine` runner rename; final doc/spec/tests.cue sweep.

## Deferred follow-up (after the slices)

**Rethink what "component" means now that infra `init` and app `init`/`discover` are one
command, and streamline the `scaffold` package accordingly.** The `baseline.Mandatory` /
`Defaults` / picker model and the `Analyze`/`AnalyzeInit` split were built when infra-init
was its own command; with detection by repo-name and a unified `init`, the component model
and the two-plan split likely collapse. Its own design pass — do not fold into the mechanical
slices.
