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
it was written). No template; write whatever shape fits.

## Lifecycle

Disposable. Edit, rewrite, or delete freely. When exploration settles into a ruling or a
design, promote the durable claim up to `../decisions/` or `../spec/`; what remains here is
the raw working material. Nothing else should depend on a scratch file continuing to exist.

## Index

Newest first. Scratch is disposable, so this list may lag — the directory is the truth.

**Live** — work in flight or current entry point:

- [2026-07-09 — Resume breadcrumb (latest)](2026-07-09-resume.md)
- [2026-07-09 — fx handoff: slog LogValuer resolving sink](2026-07-09-fx-handoff-slog-logvaluer-sink.md)
- [2026-07-08 — Project-centric builder refactor plan (pending)](2026-07-08-builder-refactor-plan.md)

**Retained for provenance** — superseded content, but cited as the evidentiary record by a
frozen decision or spec; kept so those links don't dangle:

- [2026-07-05 — Resume breadcrumb](2026-07-05-resume.md) (oras-retirement ADR)
- [2026-06-29 — Platform-as-CI architecture design](2026-06-29-platform-as-ci-design.md)
  (source for `spec/platform-server.md`)
- [2026-06-29 — Builders reshape design pass (#4)](2026-06-29-builders-reshape-design.md)
  (oras-retirement ADR)
- [2026-06-19 — D3b4 baseline design prep](2026-06-19-d3b4-baseline-design-prep.md)
  (dagger-engine ADR + vendor)
- [2026-06-17 — Slice 1 open questions](2026-06-17-slice1-open-questions.md) (appliance ADR)
- [2026-06-16 — platformv2 implementation plan](2026-06-16-platformv2-implementation-plan.md)
  (appliance + render-pure-function ADRs)
