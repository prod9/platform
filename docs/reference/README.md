# Reference

**Lookup material** — facts a reader scans to do something correctly: API
signatures, CLI flags, config keys, environment variables, schemas, error
codes, glossaries, curated external links. Answers "what exactly is X?"

Reference is for scanning, not reading start to finish. A task walkthrough is
`../guides/`. Prose explaining how or why the system works is `../spec/` —
architecture *overview* is design (`../spec/`); a schema or interface *table* is
reference (here).

## Format

One file per subject: `<slug>.md` (no date prefix — reference describes a thing,
not a moment). Favor tables and lists over prose; keep entries skimmable. Update
in place — reference always reflects the current surface.

## Index

- [`dagger-engine.md`](dagger-engine.md) — Dagger engine capabilities & deployment: SDK pin,
  the connect call, the single-engine/many-sessions model, runtime requirements, deployment
  topologies, and the load-balancer pitfall.
