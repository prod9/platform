# Terminology lexicon — one word, one concept
- **Date:** 2026-07-11
- **PR:** manual
- **Status:** accepted

## Decision

Eleven naming rulings from a full-project terminology pass. Each pins a word to exactly one
concept (or records why an apparent collision is fine). Docs conform now; code renames ride
the framework-refactor migration plan (its "Terminology renames" addendum), never as a
separate churn pass.

## The lexicon

1. **component** — reserved for infra render-ables only: an `apps/` entry and its
   `k8s/<component>/` output (one concept, two stages). The releases subsystem's semver
   "component" renames to **bump** vocabulary: `Bump` type, `BumpAny`/`BumpPatch`/
   `BumpMinor`/`BumpMajor`; CLI flags `-p`/`-m`/`--major` unchanged. CLAUDE.md's
   commit-prefix "code component" is conventions prose in a different register — untouched.
2. **strategy** — release-naming strategy (`semver`/`datestamp`/`timestamp`/`latest`),
   full stop. A framework *seeds the `strategy` value* at scaffold time — same concept,
   different moment; docs never say "scaffold strategy" or "strategy seed" as if it were a
   second kind. Runtime `Framework.Strategy()` methods and `strategy` as a package name
   stay banned (frameworks/architecture specs).
3. **render** — committed git → manifest tree, only (`platform render`, `gitops.Render`,
   `renderCue`/`renderDirectives`). Install-time file templating is *scaffolding*
   vocabulary (it moves into `framework/scaffold/` anyway). `Release.Render` (prints a
   changelog) renames to **`Changelog()`**. The DSL's internal string rendering folds into
   its existing **resolve** vocabulary.
4. **engine** — `engine/` keeps the word (it wraps the Dagger engine). The DSL
   interpreter's internal `engine` struct renames to **`interpreter`**. Prose for
   `cuelang.org/go` says **"linked CUE evaluator"**, not "engine".
5. **export** — no change. Cmd `export` (image tarball) and "CUE export" (CUE's own tool
   vocabulary) never meet on one surface.
6. **defaults** — no change, all sites. "Defaults" is site-local with no global concept:
   the `defaults/` CUE package **is** behavior-true (in CUE unification `#Basics` is the
   default base — unset fields merge its values in), Go `ProjectDefaults`/`assignDefaults`
   is toml defaulting, seeded vars are seeded vars. Each is clear where it appears.
7. **framework (toml key)** — the `[modules]` key is **`framework = "go/basic"`**. The old
   `builder` key stays readable as a **deprecated alias** (a lot of existing consumers);
   reading it emits a deprecation note. Scaffolding and docs write only `framework`.
8. **Layout / Class** — `Layout` keeps its name ("workspace layout" is natural; nothing
   competes for the word). **`Class()` is deleted from the `Framework` contract** (unused,
   dead surface); the native/bytecode/interpreted/static/custom taxonomy survives as
   descriptive prose in the frameworks spec — families of frameworks by runtime shape, not
   a contract method.
9. **platform** — unqualified "platform" in docs means the product, always. The arch
   concept is always written **arch** (`local_arch`/`publish_arch` already chose the word):
   `BuildUnit.Platform` renames to **`BuildUnit.Arch`**. The deprecated `platform` toml
   arch key stays deprecated, documented once. The remaining brand overload (binary,
   launcher, `platform.toml`, `.platform` extension) is accepted cost — renames buy
   nothing.
10. **vestigial vocabulary** — deleted: `deploy` in `project.go` comments and the default
    `Excludes` entry (no-deploy-verb ADR), the dead `environments` key in the repo's own
    committed `platform.toml` (no struct field reads it), `FindBuilder` in docs →
    **`FindFramework`** (code follows via the migration plan).
11. **minor collisions** — `baseline.Install()` renames to what it does (returns file
    bytes; it installs nothing). The scaffolded executable script is the **launcher**, one
    word everywhere. `Generate` (releases vs project) stands — different packages, Go
    idiom, never co-visible. `--force` stands with the rule pinned: **`--force` overrides
    a safety refusal** (release: dirty worktree; init: would-overwrite), **`ALWAYS_YES`
    answers prompts** — orthogonal axes, never substitutes. "moving latest" is one concept
    across its three surfaces (`latest` strategy, `Ops.Tag` default, infra
    `strategy = "latest"` seed): the moving pointer lives in the registry, never in git —
    same word everywhere is synergy, not collision.

## Rationale

The project's words had drifted into overloads ("component" carried five meanings,
"render" five, "engine" three) and near-misses that a human skims past but a less-capable
LLM reliably trips on. The fix is one-word-one-concept, decided once here so it isn't
re-litigated per file. Where a collision survived scrutiny (defaults, export, Generate,
--force, latest), the entry records *why it's fine* — that's as load-bearing as the
renames, since those are exactly the words a future pass would "clean up" wrongly.

Provenance: rulings collected 2-by-2 in-session; working plan at
[`../scratch/2026-07-11-terminology-docs-audit-plan.md`](../scratch/2026-07-11-terminology-docs-audit-plan.md).
