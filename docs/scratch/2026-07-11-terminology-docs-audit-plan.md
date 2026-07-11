<!-- not spec/decision because: working plan for an in-flight docs pass; rulings it produces
     land as a lexicon ADR + spec edits, then this file is done -->

# Terminology pass + spec audit — plan (2026-07-11)

Two ordered passes over every durable instruction surface (specs, decisions, CLAUDE.md,
indexes, vendor/guides). Goal: maximum clarity for humans **and** less-capable LLMs.
Terminology rules first — its rulings feed the audit, otherwise the audit polishes prose
that's about to be re-worded.

**Ground rule:** code↔docs drift that the locked framework refactor already fixes
(`builder/`→`framework/`, `IsInfra` alive, `baseline/`+`scaffold/` intact, `ScaffoldSpec`
shape, missing `framework/scaffold/`) is **excluded** — the specs correctly teach the
target; those stay in the
[07-08 migration plan](2026-07-08-builder-refactor-plan.md). Both passes cover only words
wrong *in the target design itself* and docs that contradict each other.

**Constraints:** no commits (one reviewable diff vs `e3fa5c6` stands). Code renames beyond
the locked refactor are **queued as an addendum to the migration plan**, never interleaved
with the uncommitted diff. Docs edits land immediately.

## Pass 1 — Terminology

Deliverable: one lexicon ADR recording each ruling; docs updated to the chosen words in the
same pass; code-rename addendum appended to the migration plan.

**RULED 2026-07-11 — all 11 clusters decided and applied to docs.** The full rulings live in
the [terminology lexicon ADR](../decisions/2026-07-11-terminology-lexicon.md); code legs in
the [migration plan's addendum](2026-07-08-builder-refactor-plan.md). Summary:

| #  | Cluster                | Ruling |
|----|------------------------|--------|
| 1  | `component`            | Reserved for infra render-ables; releases → `Bump` vocab (`BumpAny/Patch/Minor/Major`), flags unchanged |
| 2  | `strategy`             | Release-naming only; docs say "seeds the `strategy` value", never "strategy seed" as a second kind |
| 3  | `render`               | Committed git → manifest tree only; `Release.Render`→`Changelog()`, install templating = scaffold vocab, DSL→resolve |
| 4  | `engine`               | `engine/` keeps it; DSL struct→`interpreter`; prose "linked CUE evaluator" |
| 5  | `export`               | No change — never co-visible |
| 6  | `defaults` / `#Basics` | No change — site-local, behavior-true (CUE merge base), no global concept |
| 7  | toml key `builder`     | Rename to `framework`; `builder` stays a deprecated read-alias (many existing consumers) |
| 8  | `Layout` / `Class`     | `Layout` keeps; `Class()` deleted (authorized), taxonomy demoted to prose ("runtime shape families") |
| 9  | `platform` overload    | `BuildUnit.Platform`→`Arch`; bare "platform" = product, arch concept always "arch"; brand cost accepted |
| 10 | Vestigial vocab        | Delete `deploy` comment+Excludes, dead `environments` key; `FindBuilder`→`FindFramework` |
| 11 | Minor collisions       | `Install()`→named for what it does; artifact = "launcher"; `Generate` no change; `--force` = safety-gate override vs `ALWAYS_YES` = prompt driver (pinned); "moving latest" = one concept, no rename |

## Pass 2 — Spec/docs audit (after rulings)

Fix list, by severity:

### CLAUDE.md (worst surface)

`### Packages` teaches the pre-refactor shape as current, plus outright falsehoods vs the
locked design:

- Install-time picker (`OptionalMultiSelect`/`Defaults`) described as live — contradicts
  scaffolding.md:61 and the 07-11 ADR ("installs unconditionally").
- `gitctx` "environment tags (force-updated, force-pushed)" — contradicts releases.md:100-103
  (`latest` cuts **no git tag**; moving ref is a registry concern).
- `scaffold/` section says "`--force` applies unprompted" — contradicts its own Conventions
  block (`ALWAYS_YES=1`, `--force` = overwrite disposition).
- `releases/` list omits `latest`; builder list omits `Infra`; Interface omits `Scaffold`.

Fix: rewrite package sections to the locked shape + one-line "code mid-migration, see 07-08
plan" banner.

### Unmarked supersessions in decisions/

- **2026-06-18 render-routing** — teaches dead `baseline.Select` + `@variant`/`+flag` marker
  grammar; status "accepted", no banner; README index unannotated.
- **2026-06-17 appliance** — cited as authoritative by 4 specs, yet teaches removed
  `bootstrap` verb + `bootstrapper/` package (and `ops render`).
- **2026-07-05 delivery-verbs** — cited everywhere as live, written in `ops publish`/`ops
  render` vocabulary flattened the same day by the oras ADR; no forward note.

Fix pattern: partial-supersession banner (rulings stand, mechanics marked historical) +
decisions/README annotation.

### Spec-to-spec drift

- platform.md step heading "**Deploy (gated)**"; config-allocation.md "## Deploy flow" —
  rename headings ("no deploy verb" is the design's loudest rule).
- "deploy history" (config-allocation.md:22) vs "delivery history" (platform.md:65,
  platform-server.md) — unify on one term.
- platform-server.md `core` layer name collides with architecture.md's ban on `core/` as a
  package; also stale "builders reshape" wording.
- engine.md links `builder/unit.go` while marked "implemented" — annotate now, path follows
  the code rename (already tracked deferred in the 07-11 resume).

### Index / link hygiene

- scratch/README: "(latest)" points at 07-10; 07-11 resume missing entirely.
- vendor/{nginx-gateway-fabric-install,dagger-engine}.md + guides/before-going-public.md
  link into `baseline/`/`builder/` — code-true today; add one-line "path follows the
  framework rename" notes.

### Verification gate (end of Pass 2)

- Grep gate: `deploy` (as verb), `ops render|ops publish`, `bootstrap`, old filenames — only
  marked-historical hits allowed.
- Link-check all doc cross-references.
- Cold re-read of CLAUDE.md as "would a weaker LLM misread this".

Clean bill (verified, no action): spec/ + decisions/ README indexes in sync; no dangling
`builders.md`/`scaffold-baseline.md` refs; older ADRs properly bannered.

## Sequencing

Pass 1 rulings → lexicon ADR + doc word-edits → Pass 2 fixes → verification gate. Cluster 7
(toml key) + the code-rename addendum feed the migration slices, which remain the next
implementation task after this docs effort.

## Status (2026-07-11)

**Pass 1 DONE:** lexicon ADR written + indexed, code-rename addendum appended to the
migration plan, all spec/ word-edits applied (frameworks, architecture, releases,
scaffolding, platform, platform-server, config-allocation, manifest-patch-dsl), straggler
grep clean. Intentional leftovers: frameworks.md's legacy-`builder`-alias mention (that's
the ruling), engine.md's `builder/unit.go` link (code-true; Pass 2 annotates, path follows
the code rename).

**Pass 2 DONE (2026-07-11, later session):** CLAUDE.md package sections rewritten to the
locked shape + mid-migration banner; partial-supersession banners on the 06-18, 06-17, and
07-05 delivery-verbs ADRs + README index annotations; platform.md "Deploy (gated)" →
"Commit the new ref (gated)"; config-allocation "Deploy flow" → "Delivery flow" +
"delivery history" unified; platform-server `core` layer renamed to "the shared packages";
engine.md `builder/unit.go` link annotated; vendor/guides code-path notes added; loose
"builder" prose in troubleshooting-build-cache reworded. scratch/README was already
current. Verification gate ran clean: deploy/bootstrap/ops-prefix hits are all
rule-statements or defined concepts; link-check clean except pre-existing breaks inside
old scratch files (disposable by charter); CLAUDE.md cold re-read ok. **This plan is
complete** — next task is the migration slices in the
[07-08 plan](2026-07-08-builder-refactor-plan.md).
