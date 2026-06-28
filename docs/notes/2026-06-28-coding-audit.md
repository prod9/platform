# Coding audit — general-coding + go-coding (2026-06-28)

Full-codebase audit. Every sentence of `general-coding` and `go-coding` is treated as a
binding command; each is atomized into an ID'd, checkable rule below. A violation is a HARD
BLOCKER — fixed in-tree (committed, not pushed) when the fix can be safely defaulted, or
logged to `.afk.log` when it can't (ambiguous intent / behavior-change risk / chakrit's call
/ envelope boundary).

Scope: all `*.go` outside `testbeds/`, plus shell (`test.sh`, `testbed.sh`, embedded
`platform` template). Module is **go1.25.5** — `new(expr)` (Go 1.26) rules are N/A and flagged
where they'd otherwise apply.

---

## Atomic rubric — general-coding (GC-*)

### Naming
- **GC-N1** Names unambiguous at the common callsite.
- **GC-N2** Never repeat namespace context (`auth.Token`, not `auth.AuthToken`).
- **GC-N3** If imported bare, the name includes enough context to stand alone.
- **GC-N4** No grab-bag modules: `util`/`utils`/`common`/`core`/`helpers`/`helper`/`misc`/
  `extras`/`shared`. (Exception: runtime/platform's own convention.)
- **GC-N5** Name the package for its responsibility (`stringutil.Trim`, not `util.TrimString`).

### Clarity
- **GC-C1** Name each branch before combining; never chain `or`/`??` across multiple value sources.
- **GC-C2** Deep nesting is a bug; 4 levels hard max; flatten with guards / named helpers / named vars.

### DRY
- **GC-D1** Extract only when the abstraction has a meaningful name that raises the level; "the
  stuff they all do" is not a name.
- **GC-D2** Never DRY across module boundaries; shared concepts get their own module.

### Types
- **GC-T1** Invariants in types, never runtime checks; sum types over stringly-typed flags; enums
  not magic constants.
- **GC-T2** No escape hatches: no `any`/`interface{}`/unchecked casts outside serialization boundaries.
- **GC-T3** Fix at the source, never the failure point; don't widen types or swap compile errors
  for runtime fallbacks.
- **GC-T4** Validate at boundaries; trust the interior.
- **GC-T5** Narrow parameter types to the strictest that works; returns as specific as callers need.
- **GC-T6** Make invalid states unrepresentable; prefer zero values to nullables when "not set"
  isn't semantically distinct.

### Functional style
- **GC-F1** Pure functions by default; side effects at the edges only.
- **GC-F2** Immutability by default; mutate only when perf or idiom forces it.
- **GC-F3** Pipelines over accumulator loops; name intermediate steps past 3 stages.
- **GC-F4** Composition over inheritance; configuration over conditionals.

### Function shape
- **GC-S1** Three-act shape: setups/guards top, crux middle, postconditions/postprocessing end
  (grouped by blank lines).

### Code Typography — Proximity
- **GC-P1** Related items physically adjacent at every scope; layout mirrors conceptual structure.
- **GC-P2** Statements doing one step cluster together.
- **GC-P3** Public entry points first; private helpers grouped by role.
- **GC-P4** Group by domain; don't interleave with unrelated code.
- **GC-P5** When file order doesn't match conceptual grouping, reorder the file.

### Code Typography — Whitespace
- **GC-W1** Blank lines split sequences into phases.
- **GC-W2** Indentation makes hierarchy/nesting legible.
- **GC-W3** No misused whitespace: no random blanks, no missing breaks, no inconsistent indent.

### Code Typography — Contrast
- **GC-X1** Semantically different operations look different at the callsite.
- **GC-X2** Split sibling operations that diverge in effect into distinct named versions.
- **GC-X3** Mark the surprising/destructive one with a visual signal (suffix/prefix/adjective).
- **GC-X4** Never collapse distinct behaviors behind a boolean flag.
- **GC-X5** Avoid synonyms that kill contrast (e.g. `find` vs `load` with different error semantics).

### API Shape
- **GC-A1** Push caller-context out of the callee; no bool telling the callee "what kind of caller am I".
- **GC-A2** Replace variant-discriminator bools with a sum type.
- **GC-A3** Heuristic: `is_*`/`has_*`/`should_*` bools, or bools describing a *kind*, are
  discriminators in disguise.

### Trust Boundaries
- **GC-B1** Default-deny; whitelist never denylist on untrusted input.
- **GC-B2** Implementation economy never weakens a boundary.
- **GC-B3** Beware the lean-denylist rationalization.

### Shell
- **GC-SH1** Run `shellcheck`; clear every warning.
- **GC-SH2** Target POSIX `sh`; bashism only when it earns its place, declared in the shebang.
- **GC-SH3** Optimize for the next reader, not line count; split clever pipelines, name stages.

### Unit-of-work (mutations)
- **GC-U1** Mutations use a unit-of-work struct (params as fields, single `run`) over heavier abstractions.
- **GC-U2** `run` takes only a session/context — everything else on the struct.
- **GC-U3** Group units in a dedicated dir (`actions/`, `commands/`).
- **GC-U4** Verb-first file names matching the struct.
- **GC-U5** One owner per mutation; never inline a mutation another unit owns.
- **GC-U6** Commands are pure composition: N mutations = N unit calls, never N−1 + one inline.

### Testing
- **GC-TE1** Tests before implementation.
- **GC-TE2** Cover key branches, boundary cases, failure modes (honest, not exhaustive).
- **GC-TE3** Earn a meaningful red — a logic-assertion failure, not compile/"missing fn".
- **GC-TE4** No tautological tests; test behavior with logic/branching/composition.

### Git
- **GC-G1** One logical change per commit.
- **GC-G2** Commit message includes the *why*.
- **GC-G3** Polish lands as follow-up commits, not amends.
- **GC-G4** Never force-push without approval.
- **GC-G5** No destructive rewrites without approval.
- **GC-G6** Verify staged set before committing.

### Env-config
- **GC-E1** `.env` committed (working defaults). **GC-E2** `.env.local` gitignored.
- **GC-E3** Load `.env` then `.env.local`; local wins; missing skip.
- **GC-E4** Production secrets never in repo. **GC-E5** Minimal loader, no heavy dotenv lib.
- **GC-E6** No `.env.example`. **GC-E7** No `.env.production`. **GC-E8** Don't gitignore `.env`.

### Dependencies
- **GC-DEP1** Minimize import surface. **GC-DEP2** Prefer small/focused/stable over heavy.
- **GC-DEP3** Prioritize fast builds. **GC-DEP4** Measure twice before adding a dep.

---

## Atomic rubric — go-coding (GO-*)

### Nil avoidance
- **GO-N1** Prefer zero values over `*T`; `""`/`0`/`false` as the absent sentinel.
- **GO-N2** DB: `NOT NULL DEFAULT` over nullable columns.
- **GO-N3** `*T` in a struct is a smell; reserve for true tri-state or linked/tree node types.

### `new(expr)` (Go 1.26) — **N/A on go1.25.5**
- **GO-NE1/2/3** Not applicable until the module moves to 1.26. Audit only that no premature
  `new(expr)` slipped in, and that pointer fields aren't justified by it.

### Full-object updates
- **GO-FU1** Send every field / write every column; no pointer fields, no dynamic SQL, no sent-vs-not ambiguity.
- **GO-FU2** Too-large object → decompose into wholesale-replaced sub-structs, embedded back.
- **GO-FU3** Each piece gets its own update endpoint.

### Gotchas
- **GO-GP1** No non-constant printf format strings (`fmt.Print(msg)` or `fmt.Printf("%s", msg)`).
- **GO-GE1** `//go:embed` flush against the slashes (no space); pattern matches ≥1 file. Same for
  all `//go:` directives.

### Reach for the compiler
- **GO-RC1** gopls-backed tools first (scope-aware) over `Read`+`Edit`/`grep`/`sed`. (Process rule.)

---

## Findings ledger

Format: `RULE | file:line | severity | disposition`. Severity: **V**iolation / **B**orderline.
Disposition: FIXED `<commit>` / LOGGED (blocker) / WONTFIX `<reason>`.

### Shell (agent E) — complete
All scripts pass `shellcheck -s sh`. Testbed `platform` copies are vendored generated
artifacts (identical to template), excluded.

- GC-SH3 | scripts/docs-site-deploy.sh:12 | B | embedded `$(git subtree split …)` inside the
  push refspec — unnamed clever stage | FIXED — hoisted to `site_sha` var.

### dsl + gitops + baseline (agent C) — complete
gitops/ and baseline/ clean. All 5 findings in dsl/. Agent tagged all LOG; on review three
are safely defaultable (fixed), two are genuine chakrit-calls (logged).

- GC-X4 | dsl/walk.go:127 | V | `Append(…, unique bool)` collapses two behaviors behind a bool —
  inconsistent with the package's own set/set-if-absent split | FIXED — split into `Append` +
  `AppendIfAbsent` (latter delegates the append to `Append`, one owner); callsites in parse.go.
- GC-T4 | dsl/walk.go:89 | V | `Remove` panics on empty path; `Get`/`Set` both guard it | FIXED —
  added length guard returning an error.
- GC-C2 | dsl/parse.go:450 | V | `walkFocus` nests ~6 levels past the 4-level max | FIXED —
  extracted `focusStep(node, seg)`; walkFocus now flat (for/for/call).
- GC-A3 | dsl/parse.go:297 | B | `Append(…, true/false)` bool-discriminator callsites | FIXED —
  subsumed by the GC-X4 split.
- GC-B1 | dsl/io.go:138 | B | `checkRelPath` framed as denylist | LOGGED — actually `filepath.Clean`
  + `..`-containment (structural, not a signature denylist); defensible. Recommend `filepath.IsLocal`
  as a clarity hardening; not auto-rewriting trust-boundary code unsupervised.

### builder (agent A) — complete
SAFE fixes landed in `builder: Whitespace + flatten…` (45e43d5). Rest are LOG.

- GC-W3 ×3 | base.go:62, go_shared.go:12, pnpm_workspace.go:60 | FIXED (blank-line spacing).
- GC-C2 | gowork.go:16 ParseFile else-nesting | FIXED (early-return).
- GC-F1 | gowork.go:29 ParseReader double-close | FIXED (caller owns reader).
- GC-S1/T3 | fileutil.go:23 WalkSubdirs err-after-deref | FIXED (err checked first).
- GC-N4 | builder/fileutil naming | LOGGED (scoped-name borderline; + collides with internal/fileutil).
- GO-N3 | unit.go:28 `Port *int` | LOGGED (mirrors project.Module.Port; → int ripples cross-pkg).
- GC-D1 ×3 | attempt.go:38, go_basic.go:38, pnpm_basic.go:42 | LOGGED — dedupe targets; fold into #4 builders reshape.
- GC-D1/bug | dockerfile.go:56 dead `opts.BuildArgs` (Env build-args silently dropped) | LOGGED — real bug, needs intent decision.
- GC-X1 | pnpm_static.go:70 returns unsynced | LOGGED — fold into #4 run-stage normalization.

### engine + cmd (agent B) — complete
All LOG (judgment/behavior/design); none safely defaultable.

- GC-T2 | engine/engine.go:63 FromContext panic | LOGGED — intended Must/Lookup pair (LookupFromContext is the checked variant); recommend a doc note only.
- GC-N1 ×2 | cmd/discover.go:44, cmd/preview.go:101 var-shadow | FIXED (renamed bld / custom) — landed in afcfd30.
- GC-T1 | cmd/preview.go:94 `previewPort <= 1` magic | LOGGED (ignores `-p 1`; → `<= 0` or named const).
- GC-F1 | cmd/preview.go:119 data race (goroutine write + sleep-sync) | LOGGED — real bug, needs channel sync.
- GC-T1 | cmd/release.go:41 three bool flags | LOGGED — → single `--bump=patch|minor|major` enum.
- GC-U6 | cmd/publish.go:66 + deploy.go:92 inline tag mutation | LOGGED — fold into #4 (carry tag on attempt).
- GC-X1 | cmd/publish.go:79 exits 0 on result error | LOGGED — behavior bug; recommend non-zero exit.
- GC-D1 | cmd/build.go:41 result-loop dup ×4 | LOGGED — shared reportResults/exitOnError helper.

### project + releases + bootstrapper + internal (agent D + me) — complete
GC-S1 else-drops + GC-W3 landed in 0c3b9a7 and afcfd30.

- GC-S1 ×8 | project.go:94, resolve_path.go:17, bootstrapper.go:45, collection.go:66/78,
  releases.go:102, semver.go:32, timeouts.go:40 | FIXED.
- GC-W3 | gitctx.go | FIXED (gofmt). GC-N1 | timeouts.go:12 | FIXED (blank assertions).
- GC-S1/bug | plog.go:40 Logger() recursion | FIXED (return logger).
- GC-F2 | resolve_path.go:12 PlatformFilename var→const | WONTFIX — address-taken for the `-f`
  flag in main.go:30; mutable flag-var idiom, not a never-reassigned const. False positive.
- GC-T3 | releases.go:176 listCommits swallows git err → nil | LOGGED — recommend return err (behavior change).
- GC-D1 | semver.go:56 unused ErrBadVersionComponent sentinel | LOGGED — wire it or drop it.
- GO-N3 | project.go:49 `Port *int` | LOGGED — the source of the unit.go:28 mirror; → int.

## Disposition tally
FIXED & committed: shell ×1, dsl ×3, builder ×6, project/releases ×8, internal/cmd ×6 = **24**.
LOGGED (chakrit's calls, see .afk.log): **15**. WONTFIX (false positive): **1**.
