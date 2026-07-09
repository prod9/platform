# Notes

**Impermanent durable artifacts** — research dumps, surveys, drafts,
transcripts, exploratory write-ups, feature-request bodies. Anything an
agent or human produces that's worth preserving for context, but whose
claims are not expected to remain current.

If the artifact is a ruling on a question, it's a decision — `../decisions/`.
Design or architecture is `../spec/`; reference/lookup material is
`../reference/`; a task walkthrough is `../guides/`.

## Format

One file per artifact: `YYYY-MM-DD-slug.md` (the date matters because
notes are about the moment they were written). No required template —
write whatever shape fits the content.

## Lifecycle

Notes are disposable. Edit them, rewrite them, delete them — whichever
fits. They're a snapshot of past thinking, not policy and not a claim
about the present. If a note has become misleading or noisy, removing
it is fine. Nothing else in the project should depend on a specific
note still existing.

## Index

Newest first. Notes are disposable, so this list may lag — the directory is the truth.

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
  (dagger-engine ADR + reference)
- [2026-06-17 — Slice 1 open questions](2026-06-17-slice1-open-questions.md) (appliance ADR)
- [2026-06-16 — platformv2 implementation plan](2026-06-16-platformv2-implementation-plan.md)
  (appliance + render-pure-function ADRs)
