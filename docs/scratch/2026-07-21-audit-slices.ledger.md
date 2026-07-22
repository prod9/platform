<!-- not spec/decision because: a 1-by-1 walk ledger for one audit pass; the durable
output is the commits and docs/guides/go-coding-laws.md -->

# Audit slices — 1-by-1 ledger (2026-07-21)

Findings numbered per [2026-07-21-audit-changes.md](2026-07-21-audit-changes.md).

## Self-resolved (vetoable, presented 2026-07-21)

| Slice | Resolution | Derivation |
|-------|------------|------------|
| A (#106-109) | do it | go-coding Testing: require not assert; never hand-rolled reflect |
| B (#91,92,94,99,100) | do it | general-coding Abstraction; laws 8, 15 |
| C (#95,96,97,103,110) | do it | laws 2, 3, 8; Typography Proximity; Comments |

## Walk

| # | Item | Status | Ruling |
|---|------|--------|--------|
| 1 | #98 `DefaultRegistry` unexport | SETTLED | chakrit:verbatim: "Keep it exported but name it to be cue-scoped." Stays exported; rename scopes it to CUE. reading [agent]: `CUEDefaultRegistry` or `DefaultCUERegistry` — exact spelling to confirm at execution. |
| 2 | #93 `reflect` map identity in DSL executor | SETTLED | chakrit:verbatim: "a) or we could just type-assert each supported type individually which is how it was done previously, not sure why it's been changed." Kill the reflect. Preferred shape (a) scope carries owning doc index; individual type-assertion acceptable alternative. Executor state only — no grammar/verb/semantics change, so not DSL-gated. |
| 3 | #37/#38 nil `*sqlx.DB` threading | SETTLED | chakrit:verbatim: "Eliminate all of those stupid helpers that adds nothing, just rely on data.Connect directly." Both connectOrNil and connectDB die; Serve calls data.Connect at the callsite. reading [agent]: DB-less boot stays possible (installer path) — confirm at execution how the not-connected state reads once the wrappers are gone. |
| 4 | #39 `recordOutcome` swallow | KILLED (superseded) | chakrit challenged why srv/builds holds a runner at all; .ace/save.md:63-64 already lists srv/builds/runner.go as pre-v2, "to be ripped out and rebuilt" by queue slice 4. chakrit:verbatim: "I'd say delete the code RIGHT NOW after we finish w 1by1" — delete the runner rather than fix its error handling. Takes #46 (publishBuild seam) with it. Scope of the deletion (runner.go alone vs its siblings) to be presented before execution. |
| 5 | #89 mutations inlined into commands | KILLED | chakrit:verbatim: "That's stupid. those are utility calls/functions it's not part of the 'domain' to warrant an action. We can think about refactoring those later if we have more use case later." Unit-of-Work is for domain mutations; a command's own plumbing (exec, serve, export) is not domain. Revisit only on a second consumer. |

## Correction (2026-07-22)

`engine/runners.go` was written by chakrit, not by an agent. Law 19's claim that "every
commit here is agent-authored" was false and produced an invented vendor-vocabulary story
for why the file is named `runners`; the law is amended. The naming collision between
`engine/runners` (Dagger engine endpoints) and a CI-sense "runner" is still real and still
worth settling when slice 4 names its queue worker — but the *why* is unestablished.

## Correction 2 (2026-07-22) — the component is a *worker*, not a "dispatcher"

`fx.prodigy9.co/worker` already exists: `worker.Interface` (`Name`, `Run(ctx) error`), a
job registry, `WORKER_POLL`. chakrit:verbatim: "Note that fx already has built-in worker
module as well, that's why i called everything a worker." The specs were written with an
invented word ("dispatcher") and are corrected to `worker` throughout — law 12, checked
against live concepts only after being told, not before.

Three live concepts, three words, now spec'd: **engines** execute · **runners** are the
Dagger endpoints they live at · **workers** decide what to hand them.

## Design captured to spec (2026-07-22)

The trigger→build design settled in conversation this session is now in the specs, which
precede implementation per CLAUDE.md:

- [`engine.md`](../spec/engine.md) §The execution boundary — what defines an engine, the
  two scheduling decisions, no dagger verbs outside `engine/` (logic not types).
- [`platform-server.md`](../spec/platform-server.md) §Triggering a build — the four
  one-way boundaries, the record-is-the-queue rule, worker placement and naming.
