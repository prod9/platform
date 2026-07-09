# Scaffold & Baseline

Status: **design-of-record.** Covers `platform init` — the `scaffold/` plan builder and
the `baseline/` embedded cluster baseline it folds in. New code conforms; existing code
migrates toward it.

`init` (alias `scaffold`) seeds two kinds of repo from one command: an **app repo**
(`platform.toml` + executable `platform` build script) or an **infra repo** (that base
plus the embedded GitOps baseline — `apps/`, the mandatory `defaults/` package,
`cue.mod`). The split is decided by `builder.IsInfra(wd)`: an "infra" substring glob on
the directory name (`infra`, `fi-infra`, `infra-stage9` all qualify). Name is the identity
— a directory marker like `apps/` is a poor signal.

For the pipeline this feeds, see [architecture](architecture.md); scaffold-time stack
discovery lives in `builder` (the build path reads `[modules]`, never re-discovers).

## Analyze → Plan → Apply

Computing the plan is **pure** (reads only) so `init` can print and confirm it before any
mutation. Two entry points build a `Plan`; both refuse to touch the tree.

| Entry         | Repo kind | Git gate                              | Extra scaffold        |
| ------------- | --------- | ------------------------------------- | --------------------- |
| `Analyze`     | app       | **hard** — must be inside a git repo  | none                  |
| `AnalyzeInit` | infra     | none — `init` creates the repo        | `cue.mod/module.cue`  |

The app-side git gate (`validateWD` → `IsGitRepo`, walk-up for `.git`) is deliberate: the
appliance baseline is delivered through GitOps, so a non-repo app target is virtually
always a mistake. `AnalyzeInit` skips it (`validateInitWD` checks only dir-exists) —
`init` runs `git init` itself via `ensureGitRepo`, gated on `IsGitRoot` (a `.git`
**directly** in `wd`, no walk-up) so it creates a standalone repo even nested inside
another checkout.

Both entries produce the same two base files — `platform.toml` and the executable
`platform` launcher (rendered from the embedded `platform.template`, mode `0744`) — plus,
for infra, the `cue.mod` file. The infra baseline components are **not** produced by
`scaffold`: `cmd/init` renders them via `baseline.Render` and folds each in with
`Plan.AddFile`.

`Plan` carries `Files []FileChange` and `Vars []VarChange`. Each `FileChange` records
`FileWrite` vs `FileOverwrite` (decided by an existence stat at plan time) so `Print` can
warn before an overwrite and the operator sees the disposition up front. `Apply` writes,
**skipping overwrites**; `ApplyOverwrite` replaces in place. `Overwrites()` counts
replacements so `init` can prompt only when some exist.

### Non-interactive drive

Drive `init` non-interactively with **`ALWAYS_YES=1`**, not `--force`. They are
orthogonal:

- `ALWAYS_YES=1` — the fx prompt session auto-answers every prompt (the component picker
  takes its `Defaults`, `YesNo` returns yes). This is how you script an init.
- `--force` — sets `ApplyOverwrite`: **replace existing files** instead of keeping them.
  Purely about the write disposition, not about suppressing prompts.

`cmd/init` prints the plan, confirms (`apply this plan?`), then — when overwrites exist
and `--force` wasn't passed — confirms the replacement count separately. It closes by
encoding the effective parsed config (`project.Configure`, same view as `configure`) so
the operator sees the resolved result of the freshly written `platform.toml` in one shot.

### platform.toml disposition

`planProjectFile` branches on whether `platform.toml` already exists:

- **Absent** → `generateProjectFile` writes a fresh file from `project.ProjectDefaults`,
  the operator `Info` (maintainer, repository), the discovered module (`builder.Discover`
  → one `[modules]` entry; `platform/infra` also flips `strategy = "latest"` since infra
  has no versions to cut), and the seeded baseline `[ops.vars]`.
- **Present** → `mergeOpsVars` folds the baseline defaults in **textually** (see below);
  every other table, comment, and byte is preserved.

## Re-init: surgical `[ops.vars]` merge

Re-running `init` must not clobber an operator's `platform.toml`. `mergeOpsVars`
(`vars_merge.go`) merges the baseline default `[ops.vars]` **line-by-line, not by
decode/re-encode** — a round-trip through the TOML encoder would lose the operator's
comments, ordering, and formatting. The merge:

- **Appends** default keys the file lacks, inserted after the last non-blank line of the
  existing `[ops.vars]` body (or a fresh section appended to EOF when absent).
- **Preserves** keys already present — the operator's value stands untouched.
- Leaves comments, key order, and all other tables byte-for-byte.

Each default's disposition is recorded as a `VarChange` (`Appended` true = newly added,
false = operator value kept), surfaced in `Plan.Print`. Values keep their TOML type on the
append (`tomlValue`: strings quoted/escaped, bools and numbers bare).

## The flat embedded baseline

`baseline/` ships **the** cluster baseline — platform's opinion, version-locked to the
tool, not the operator's configuration (see
[opinionated-appliance-embedded-init](../decisions/2026-06-17-opinionated-appliance-embedded-init.md)).
It is **one flat list**, no marker grammar and no render-time gating (see
[flat-baseline-install-time-selection](../decisions/2026-06-22-flat-baseline-install-time-selection.md)).

`EmbeddedFiles()` returns every built-in `files/*` blob keyed by filename. Filenames are
**destination-encoded by prefix** — routing keys, not selection markers. `baseline.Render`
maps each to its repo-relative destination (dropping any `.tmpl` suffix):

| Name prefix   | Destination   | Role                                                     |
| ------------- | ------------- | -------------------------------------------------------- |
| `apps-*`      | `apps/`       | render-able components (each → an `ops render` output)   |
| `defaults-*`  | `defaults/`   | shared CUE definitions imported by `apps/` (`#Basics`)   |
| _(other)_     | repo root     | root files (e.g. `platform.toml`)                        |

### Install-time selection

Selection is **install-time**, not render-time. The operator changes their baseline by
re-running `init` (re-init → pick → bump version), so the gate belongs at install: pick
the files once, write them. `cmd/init.selectComponents` builds a checkbox list of the
optional files with `Defaults` pre-checked and drives it via `sess.OptionalMultiSelect`.
Each chosen file installs to the destination its name encodes.

- **`Defaults`** — the hard-coded working set a functioning cluster needs out of the box
  (cert-manager, flux, flux-sync, the platform app, experimental nginx-gateway). Stable
  nginx-gateway ships alongside but is off by default; the operator opts in.
- **`Mandatory`** — the `defaults/` package (`defaults-basics.cue.tmpl`), installed on
  **every** init regardless of the picker. Never offered as a choice — deselecting it
  would break every app that imports `#Basics`. A "choice" like experimental-vs-stable
  nginx-gateway is just two files with one in `Defaults`; if an operator keeps both,
  that's their call — no mutual-exclusion machinery.

### Templating

`Render` routes each selected file and renders `.tmpl` files through `text/template`
(`missingkey=error`); non-template files pass through **verbatim** (their CUE braces must
never meet the template engine). `TemplateData` fills placeholders at init time — registry
creds (prompted; password is `SensitiveStr`), `DaggerVersion` (from the linked SDK, see
below), `ModulePath`, and `OpsImage` (the flux self-sync OCI base). Output order is
deterministic.

Creds and the dagger version enter via **go-template at init time** — concrete values
written into the target repo's `defaults/basics.cue` — never CUE injection (see invariants
below).

### DefaultVars: version pins only

`DefaultVars` is the shipped `[ops.vars]`: **version pins only** (`CERT_MANAGER_VERSION`,
`FLUX_VERSION`, …). Keys are SCREAMING_SNAKE (the preferred `platform.toml` form; render
lowercases for both consumption routes). They are pure interpolation inputs — `\(var)` in
directive `download` URLs and `@tag(var)` in CUE apps. **Component selection is not a
var** — it is the install-time picker, orthogonal to the pins.

### Dagger version pin

`DaggerVersion()` reports the `dagger.io/dagger` SDK version this binary is linked against
(read from `debug.ReadBuildInfo`, honoring a `replace`). A freshly-init'd infra repo pins
`registry.dagger.io/engine:<version>` to it, so the in-cluster engine and the SDK driving
it never drift. `init` treats empty as a hard error rather than emitting a tagless engine
ref.

### cue.mod scaffold

`planCueModule` scaffolds `cue.mod/module.cue` only on a **greenfield** infra repo (no
existing module, `ModulePath` set). It pins the operator's module path, the linked CUE
engine's language version (so render never demands a newer language than it links), and
the `DefsModule`/ `DefsVersion` infra-defs dependency the baseline apps import. An
existing module is the operator's truth — read its path (`ModulePath`, `@vN` suffix
stripped), never rewritten.

## Invariants — do NOT re-litigate

- **`defaults/` is mandatory** on every infra repo. It is the home for shared definitions
  (`#Basics`: namespace + registry pull secret), imported by `apps/`. Always installed,
  never offered in the picker.
- **`apps/` is render-only.** Every top-level key under `apps/` becomes an `ops render`
  output. Shared definitions do not live here — they live in `defaults/`.
- **CUE `@tag` injection does not cross the module/package import barrier.** `@tag`/`-t`
  injection is root-package only; an imported package errors `no tag for "X"`. So registry
  creds **cannot** be `@tag`-injected into an imported `defaults/basics.cue`, and
  relocating the shared def into `apps/` to dodge this is **banned** — it breaks
  apps-is-render-only. Only names a `@tag` actually declares get injected; the rest are
  directive-only. Creds enter by go-template at init time (concrete after init), not tag
  injection. </content> </invoke>
