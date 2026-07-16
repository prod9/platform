# Scaffolding

Status: **design-of-record.** Covers `platform init` — how a fresh repo is seeded. New
code conforms; existing code migrates toward it.

Scaffolding has exactly three concerns, each with one home:

- **`framework/scaffold/`** — the one files/templating mechanism: resolve templates
  with data. Generic; no discover, no orchestration, no per-type data, no writes — the
  driver writes finished bytes.
- **The `Infra` framework** — owns the cluster baseline it scaffolds: its component set,
  version pins, and destination routing (the bytes ship in the `framework/skel`
  collection, alongside the universal launcher). There is no standalone `baseline/` package.
- **`cmd/init`** — the human orchestration: gather operator inputs → `framework.Discover`
  → `fw.Scaffold` → confirm → write.

A `Framework` is the sole owner of a project type, so what a repo scaffolds is
`fw.Scaffold`'s output — nothing branches on app-vs-infra. An **app repo** gets a
`platform.toml` + executable `platform` launcher; an **infra repo** gets that base plus the
whole GitOps baseline (`apps/`, the `defaults/` package, `cue.mod`). The difference is only
how much `Infra.Scaffold` contributes — pure polymorphism, **no `IsInfra` predicate**.

The launcher is the driver's own contribution, so the driver resolves its one hole the
same way frameworks resolve theirs: `PLATFORM_VERSION` is stamped with
`framework.PlatformVersion()` — the nearest release the running binary descends from (an
exact tag verbatim; a between-releases pseudo-version resolves to the predecessor tag it
encodes, keeping smoke goldens deterministic between cuts; nothing derivable is a hard
init error).

For the pipeline this feeds, see [architecture](architecture.md); the `Framework` contract
and scaffold-time stack discovery live in [frameworks](frameworks.md) (the build path reads
`[modules]`, never re-discovers).

## `framework/scaffold/` — the one mechanism

`framework/scaffold/` resolves a framework's contribution. It defines the two
shapes a framework returns and nothing stack-specific:

- **`scaffold.Spec`** — a framework's full contribution to a fresh repo: the
  `platform.toml` module it adds, the default `[vars]` it seeds, the files it ships
  (`[]scaffold.File`, already **resolved** — the framework fills its own holes), and the
  default `strategy` value it seeds.
- **`scaffold.File`** — one file beyond the universal `platform.toml` + launcher: `Path`
  (repo-relative, routing already applied), `Content`, `Mode`. A `.tmpl` suffix marks
  `Content` as a `text/template` the mechanism resolves (and strips) with the
  scaffold data.

**Templating rules.** `.tmpl` files resolve through `text/template` with `missingkey=error`;
non-template files pass through **verbatim** — their CUE braces must never meet the template
engine. Placeholders are filled at init time: `DaggerVersion` (from the linked SDK, below),
`MaintainerEmail` (init's universal prompt; the cluster-issuer's ACME contact),
`ModulePath` (the operator's CUE module path — from the `CUE_MOD_PREFIX` scaffold input or an
existing `cue.mod`, separate from `repository`), and `ImageBase` (the flux self-sync OCI base). Registry creds are **not**
templated — they ship as empty placeholders in committed CUE (below). Output order is
deterministic.

The mechanism is generic: it does not discover, does not orchestrate, and holds no per-type
data. Everything type-specific comes in through the `scaffold.Spec` a framework hands it.

## The `Infra` framework owns its baseline

The cluster baseline — the thing an infra repo scaffolds — is **platform's opinion**,
version-locked to the tool, not the operator's configuration (see
[opinionated-appliance-embedded-init](../decisions/2026-06-17-opinionated-appliance-embedded-init.md)).
Under the framework model it belongs to the framework that scaffolds it: the `Infra`
framework owns the baseline component set, version pins, and destination routing — the
file bytes ship in the `framework/skel` collection (see
[baseline-dissolves-into-infra-framework](../decisions/2026-07-11-baseline-dissolves-into-infra-framework.md)).

`Infra.Scaffold` returns the **full default baseline unconditionally** — one flat list, no
marker grammar and no render-time gating (see
[flat-baseline-install-time-selection](../decisions/2026-06-22-flat-baseline-install-time-selection.md)).
There is **no install-time picker**: an operator who wants a smaller baseline prunes the
committed files after init, the same hand-edit model that governs the image ref and the CUE
literals.

### Destination-encoded files

Baseline filenames are **destination-encoded by prefix** — routing keys, not selection
markers. Each maps to its repo-relative destination (dropping any `.tmpl` suffix):

| Name prefix   | Destination   | Role                                                     |
| ------------- | ------------- | -------------------------------------------------------- |
| `apps-*`      | `apps/`       | render-able components (each → a `render` output)        |
| `defaults-*`  | `defaults/`   | shared CUE definitions imported by `apps/` (`#Basics`)   |
| _(other)_     | repo root     | root files (e.g. `platform.toml`)                        |

The default working set is what a functioning cluster needs out of the box: cert-manager
(with the `ListenerSets` feature gate), flux, flux-sync (the push-driven webhook `Receiver`
+ its own `ListenerSet` and route), the platform app, the NGF gateway stack, the
host-agnostic operator `Gateway` app, and the ACME cluster-issuer. Components own their
hostnames via `ListenerSet`s (distributed-hosts shape) — the gateway app carries none. It
installs whole — selection is not an operator choice at init time.

### `DefaultVars`: interpolation inputs, not selection

The baseline's default `[vars]` are **pure interpolation inputs**: version pins
(`CERT_MANAGER_VERSION`, `FLUX_VERSION`, …) and the per-deployment ingress hosts the CUE apps
route on (`PLATFORM_HOSTNAME` for the platform app, `FLUX_HOSTNAME` for the Flux webhook
route). Keys are SCREAMING_SNAKE (the preferred `platform.toml` form; render lowercases for
both consumption routes) — `\(var)` in directive `download` URLs and `@tag(var)` in CUE apps.
**Selection is not a var** — the full baseline installs unconditionally.

### Registry creds are defaulted, not prompted

The baseline ships `#registry_username` / `#registry_password` as **empty placeholders in
committed CUE** that the operator hand-edits — consistent with the committed-literal model.
`init` prompts for neither, and they are not `@tag`-injected (see invariants). A security
edit lives in the committed repo, like every other literal.

### Dagger version pin

`DaggerVersion()` reports the `dagger.io/dagger` SDK version this binary is linked against
(read from `debug.ReadBuildInfo`, honoring a `replace`). A freshly-init'd infra repo pins
`registry.dagger.io/engine:<version>` to it, so the in-cluster engine and the SDK driving
it never drift. `init` treats empty as a hard error rather than emitting a tagless engine
ref.

### `cue.mod` scaffold

The `Infra` framework contributes `cue.mod/module.cue` on a **greenfield** infra repo (no
existing module, `ModulePath` from the `CUE_MOD_PREFIX` scaffold input). It pins the operator's
module path (their CUE namespace, deliberately distinct from the GitHub `repository`), the linked CUE
evaluator's language version (so render never demands a newer language than it links), and the
`DefsModule`/`DefsVersion` infra-defs dependency the baseline apps import. An existing
module is the operator's truth — read its path (`ModulePath`, `@vN` suffix stripped), never
rewritten.

## `cmd/init` — orchestration

`cmd/init` (alias `scaffold`) drives the flow and owns every mutation. It is
**plan-then-apply**: computing the plan reads only, so `init` prints and confirms it before
touching the tree.

1. `init` validates the target: it exists, is a directory, and is its own git repo root
   (`git.IsRoot` — a `.git` **directly** in `wd`, no walk-up, so a standalone repo nested
   inside another checkout counts only with its own `.git`).
2. `framework.Discover(wd)` resolves the owning framework, and `init` prompts for the
   inputs its `RequiredScaffoldInputs(wd)` declares.
3. `fw.Scaffold(ctx, wd, env, inputs)` — `env` is `scaffold.Env` (repository, maintainer
   email, dagger SDK version) — returns the `scaffold.Spec` —
   module, vars, files (resolved), the seeded `strategy` value. `init` computes the
   `platform.toml` disposition (below) and builds a plan.
4. It prints the plan, confirms, then writes finished bytes.

**The git precondition is uniform, not framework-set.** Platform never runs `git init` —
the operator creates the repo (or clones) first, for every framework alike: delivery is
git-based end to end, so a non-repo target is always a mistake.

The plan carries `Files []FileChange` and `Vars []VarChange`. Each `FileChange` records
`FileWrite` vs `FileOverwrite` (decided by an existence stat at plan time) so `Print` can
warn before an overwrite. `Apply` writes, **skipping overwrites**; `ApplyOverwrite` replaces
in place. `Overwrites()` counts replacements so `init` prompts only when some exist. `init`
closes by encoding the effective parsed config (`conf.Load`, same view as
`configure`) so the operator sees the resolved result of the freshly written `platform.toml`.

### Non-interactive drive

Drive `init` non-interactively with **`ALWAYS_YES=1`**, not `--force`. They are orthogonal:

- `ALWAYS_YES=1` — the fx prompt session auto-answers every prompt (`YesNo` returns yes).
  This is how you script an init.
- `--force` — sets `ApplyOverwrite`: **replace existing files** instead of keeping them.
  Purely about the write disposition, not about suppressing prompts.

### `platform.toml` disposition

- **Absent** → a fresh file is generated from `conf.ModelDefaults`, the operator `Info`
  (maintainer, repository), the framework's `scaffold.Spec` module (its `[modules]` entry and
  the `strategy` value it seeds — `Infra` seeds `strategy = "rolling"` since infra has no
  versions to cut; the CUE module path is a scaffold input (`CUE_MOD_PREFIX`), never a
  `platform.toml` key), and the seeded default `[vars]`.
- **Present** → the surgical `[vars]` merge (below) folds the baseline defaults in
  **textually**; every other table, comment, and byte is preserved.

### Re-init: surgical `[vars]` merge

Re-running `init` must not clobber an operator's `platform.toml`. The merge (owned by
`conf/`, alongside `Generate`) folds the baseline default `[vars]` in **line-by-line,
not by decode/re-encode** — a round-trip through the TOML encoder would lose the operator's
comments, ordering, and formatting. The merge:

- **Appends** default keys the file lacks, inserted after the last non-blank line of the
  existing `[vars]` body (or a fresh section appended to EOF when absent).
- **Preserves** keys already present — the operator's value stands untouched.
- Leaves comments, key order, and all other tables byte-for-byte.

Each default's disposition is recorded as a `VarChange` (`Appended` true = newly added,
false = operator value kept), surfaced in the plan. Values keep their TOML type on the
append (strings quoted/escaped, bools and numbers bare). A directive *file* edit is **not**
preserved across re-init — customization is via vars; the directive files are platform's
opinion, re-shipped whole.

## Invariants — do NOT re-litigate

- **`defaults/` is mandatory** on every infra repo. It is the home for shared definitions
  (`#Basics`: namespace + registry pull secret), imported by `apps/`. Always installed as
  part of the unconditional baseline.
- **`apps/` is render-only.** Every top-level key under `apps/` becomes a `render` output.
  Shared definitions do not live here — they live in `defaults/`.
- **CUE `@tag` injection does not cross the module/package import barrier.** `@tag`/`-t`
  injection is root-package only; an imported package errors `no tag for "X"`. So registry
  creds **cannot** be `@tag`-injected into an imported `defaults/basics.cue`, and relocating
  the shared def into `apps/` to dodge this is **banned** — it breaks apps-is-render-only.
  Creds therefore ship as empty placeholders the operator hand-edits, never injected. Only
  names a `@tag` actually declares get injected; the rest are directive-only.
