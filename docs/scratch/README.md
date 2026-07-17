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

**Live** — current entry point / work in flight:

- [2026-07-12 — Resume breadcrumb (latest)](2026-07-12-resume.md) — **start here**
- [2026-07-17 — srv API/architecture 1-by-1](2026-07-17-srv-1by1.md) — walk **in progress**;
  chakrit's standing rulings are givens, every walk item is unsettled until he closes it
- [2026-07-17 — containerd vs the Dagger engine](2026-07-17-containerd-vs-dagger-engine.md)
- [2026-07-09 — fx handoff: slog LogValuer resolving sink](2026-07-09-fx-handoff-slog-logvaluer-sink.md)

**Retained for provenance** — cited as the evidentiary record by a frozen decision or spec;
kept so those links don't dangle:

- [Prior-art digest](prior-art.md) — superseded design/plan scratch collapsed into one file
  (2026-07-12); the appliance, render-pure-function, dagger-engine, oras-retirement,
  terminology-lexicon, and monorepo decisions plus `spec/platform-server.md` cite its sections
- [2026-07-05 — Resume breadcrumb](2026-07-05-resume.md) (oras-retirement ADR)
