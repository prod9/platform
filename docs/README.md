# docs

Durable artifacts. **File by the gate below** — walk it top to bottom and stop at the first
yes. The bottom (`scratch/`) charges a toll, so nothing lands there by default.

## Where does this go?

1. A ruling you'd defend if someone reopened it? → [`decisions/`](decisions/) — dated,
   never edited.
2. Third-party facts you keep to look up (a framework, an external API/CLI)? →
   [`vendor/`](vendor/) — link-first, mark provenance.
3. A how-to — using the product *or* operating the repo? → [`guides/`](guides/) — script
   repeatable operations; the guide holds the judgment.
4. How our system is built or meant to work, including its own config/CLI surface? →
   [`spec/`](spec/).
5. None of the above — genuinely unsettled exploration → [`scratch/`](scratch/). Open with
   a one-line "not spec/decision because ___."

Each folder's README states its one test precisely. `CLAUDE.md` points here as the index.

## Spec-first — the spec is the most important document

`spec/` is the single source of truth for how the system works and how we intend it to
work. It is the first thing anyone — human or agent — reads to reconstruct the design, so a
stale spec makes every downstream reconstruction stale. Not hypothetical: a decision
recorded only in `scratch/` while the spec kept describing the superseded design got
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
