# Scratch

**Unsettled exploration** — research dumps, surveys, investigations, drafts, transcripts,
resume breadcrumbs: thinking-in-progress whose claims are not expected to stay current.
This is the residual home. Material lands here only when it fits nothing above it in the
routing gate — never as a default.

Belongs here *only* if it is genuinely exploratory. A ruling is `../decisions/`; current or
intended design (including our own exact surface) is `../spec/`; third-party lookup is
`../vendor/`; a task walkthrough is `../guides/`.

**Toll.** Open every new scratch file with one line naming why it is not spec or a decision:

```
<!-- not spec/decision because: still exploring; no ruling made yet -->
```

If you cannot write that line honestly, the artifact belongs in one of those folders — put
it there instead.

## Format

One file per artifact: `YYYY-MM-DD-slug.md` (the date matters — scratch is about the moment
it was written). No template; write whatever shape fits. Exception — `LOG.md`, the
append-only session journal, is undated by design (a current surface, not a moment-in-time
artifact). The live *state* trail (current truth + ruling ledger) is no longer here: it
moved to gitignored `.ace/save.md` + `.ace/save.ledger.md` (see CLAUDE.md "Session trail").

## Lifecycle

Disposable. Edit, rewrite, or delete freely. When exploration settles into a ruling or a
design, promote the durable claim up to `../decisions/` or `../spec/`; what remains here is
the raw working material. Nothing else should depend on a scratch file continuing to exist.

## Index

Newest first. Scratch is disposable, so this list may lag — the directory is the truth.

**Live state** lives in gitignored `.ace/` now, not here — `.ace/save.md` (current truth,
**start there**) and `.ace/save.ledger.md` (ruling ledger; walk statuses live nowhere else).

**Live** — committed work in flight:

- [LOG](LOG.md) — append-only session journal (archaeology; never read on resume)
- [2026-07-17 — trail fix plan](2026-07-17-trail-fix-plan.md) — why the state trail split
  from the journal (schema, provenance, disciplines)
- [2026-07-17 — srv API/architecture 1-by-1](2026-07-17-srv-1by1.md) — frozen context for
  the ledger (derivations, evidence)
- [2026-07-18 — srv RBAC & observability authz](2026-07-18-srv-rbac-observability.md) —
  item 11 full record: zero-RBAC confirmed; cluster-view via GitHub rights + cluster-side
  provenance discovery + a fat caching session (SETTLED; graduates to spec/ + ADR note)
- [2026-07-19 — builder lifecycle structure](2026-07-19-builder-lifecycle-options.md) —
  framework emits a serializable `Plan` → `Execute` with `.Sync()` phase boundaries + a
  host-side observer feeding the event-sourced reconciler; one-install-per-cluster ruled
- [2026-07-18 — TO-DO: de-confuse "flux webhook"](2026-07-18-flux-webhook-deconfusion-tasks.md)
  — **pending task list** (GitHub→Flux vs Flux→srv; doc edits + delete the wrong ADR); delete
  when done
- [2026-07-17 — containerd vs the Dagger engine](2026-07-17-containerd-vs-dagger-engine.md)
- [2026-07-09 — fx handoff: slog LogValuer resolving sink](2026-07-09-fx-handoff-slog-logvaluer-sink.md)

**Retained for provenance** — cited as the evidentiary record by a frozen decision or spec;
kept so those links don't dangle:

- [Prior-art digest](prior-art.md) — superseded design/plan scratch collapsed into one file
  (2026-07-12); the appliance, render-pure-function, dagger-engine, oras-retirement,
  terminology-lexicon, and monorepo decisions plus `spec/platform-server.md` cite its sections
- [2026-07-05 — Resume breadcrumb](2026-07-05-resume.md) (oras-retirement ADR)
