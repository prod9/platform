# docs

Durable artifacts, in two clusters that sort on two different axes — on purpose.

## Usage — how to use what this repo produces (sorted by type)

- [`guides/`](guides/) — task-oriented how-to and getting-started. *How do I do
  X?*
- [`reference/`](reference/) — lookup facts: API, CLI, config, schemas,
  glossaries, links. *What exactly is X?*

## Design record — how and why this repo is built (sorted by permanence)

- [`spec/`](spec/) — design and architecture; intent and how-it-works. *What we
  intend, and how it fits together.* Living.
- [`decisions/`](decisions/) — dated ADRs. *What we decided, and why.* Frozen;
  superseded by newer decisions, never edited.
- [`notes/`](notes/) — research, drafts, exploration. *What we explored.*
  Disposable.

When unsure: understand-the-system prose → `spec/`; look-it-up facts →
`reference/`; do-a-task steps → `guides/`; a defended ruling → `decisions/`;
everything else → `notes/` (the default).

## Spec-first — the spec is the most important document

`spec/` is the single source of truth for how the system works and how we intend it to
work. It is the first thing anyone — human or agent — reads to reconstruct the design, so a
stale spec makes every downstream reconstruction stale. Not hypothetical: a decision
recorded only in `notes/` while the spec kept describing the superseded design got
re-litigated 3–4 times, because reconstructing from the trustworthy sources (spec + code)
re-taught the old answer every time.

So the ordering is a law, not a preference:

1. **A design change lands in `spec/` first** — before code, before any decision doc.
   Update the spec to the new/intended design even when the code hasn't caught up; mark the
   section as intended and link the ADR if one exists. Never leave the spec teaching a
   design we've already moved off.
2. **A decision doc is secondary, never a substitute.** Write it only after the spec is
   updated, and only when the ruling went against an obvious default and needs a
   re-litigation defense. It records the frozen *why* and links back to the spec; it is
   never where a reader learns current state. If someone must open `decisions/` to know how
   the system works today, the spec has failed its job.
