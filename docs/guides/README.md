# Guides

**Task-oriented usage docs** — how-to guides and getting-started walkthroughs
for whoever (human or agent) *uses* what this repo produces. Answers "how do I
accomplish X?"

A guide is goal-driven: it walks one real task start to finish. Enumerating
facts (every flag, every config key) is `../vendor/`. Explaining how the
system fits together or why it's shaped that way is `../spec/`.

## Format

One file per task: `<slug>.md` (no date prefix — a guide describes a task, not a
moment). Keep each guide to one job; link to `../vendor/` for exhaustive
detail rather than inlining it. Update in place.

## Index

- [`troubleshooting-build-cache.md`](troubleshooting-build-cache.md) — `platform clean` as
  first-line diagnosis for a "worked on a fresh checkout but not here" build failure; the
  Dagger cache-poisoning mode; why pnpm→apk is never the fix.
- [`before-going-public.md`](before-going-public.md) — the scrub checklist for when the repo
  goes public: LICENSE, firewall-id placeholder, exclude `docs/scratch/`.
- [`migration.md`](migration.md) — how a consuming repo moves across each breaking change of
  the legacy → pull-based v2 rework.
