# Decisions Log

**Point-in-time defenses against future re-litigation** — rulings made on
a specific date for a specific question, recorded so the same argument
doesn't have to be re-fought next quarter. Each entry is frozen at the
moment of decision; if a later ruling reverses it, write a new dated
decision that links back and mark the old one `superseded`.

## When to add an entry

Add a decision when **the answer goes against the obvious default** —
mainstream practice, what the agent's training data would suggest, or the
project's own prior convention. The point of the log is to capture the
*why* so future arguments don't keep re-discovering it. Examples that
warrant an entry:

- We deliberately deviate from a well-known pattern, and a future agent
  reading our code would assume we just didn't know better.
- A reviewer pushed back on a choice that we then defended; the defense
  is worth preserving.
- Two reasonable approaches were debated and one won — without the entry,
  the next debate replays from scratch.

**Don't** add a decision when the answer is already obvious or matches
the prevailing convention. If there's no future confusion to head off,
just document the result in `../spec/` and move on. A decisions log
cluttered with "we chose the obvious thing" entries makes the actual
load-bearing decisions harder to find.

If your artifact is research, a survey, a draft, a transcript, or any
exploratory write-up — that's notes, not a decision. Use `../notes/`. If
it's forward-looking design, use `../spec/`.

## Format

One file per decision: `YYYY-MM-DD-slug.md`

```markdown
# Short Title
- **Date:** YYYY-MM-DD
- **PR:** #N (or "manual")
- **Status:** accepted | superseded | revised

## Decision
One-liner.

## Rationale
Why this, and specifically why *not* the obvious alternative — that's
the part that prevents re-litigation.
```

## Statuses

- **accepted** — active, follow this decision
- **superseded** — replaced by a newer decision (link to it)
- **revised** — updated in-place with new context

## Index

Newest first.

- [2026-07-05 — Platform FHS container layout; cmd is the runtime command](2026-07-05-platform-fhs-container-layout.md)
- [2026-06-26 — Render is a pure function of committed git](2026-06-26-render-is-pure-function-of-committed-git.md)
- [2026-06-24 — Split build and server log channels](2026-06-24-split-build-and-server-log-channels.md)
- [2026-06-23 — Render via the linked CUE engine](2026-06-23-render-via-linked-cue-engine.md)
- [2026-06-22 — Flat baseline, install-time selection](2026-06-22-flat-baseline-install-time-selection.md)
- [2026-06-21 — Dagger engine: StatefulSet + TCP](2026-06-21-dagger-engine-statefulset-tcp.md)
- [2026-06-20 — DSL: focus scope, strict values](2026-06-20-dsl-focus-scope-strict-values.md)
- [2026-06-18 — Render routes .cue and .platform by extension](2026-06-18-render-routes-cue-and-platform-by-extension.md)
- [2026-06-17 — Opinionated appliance, embedded init](2026-06-17-opinionated-appliance-embedded-init.md)
- [2026-06-17 — Generic ops vars, single config](2026-06-17-generic-ops-vars-single-config.md)
- [2026-06-16 — Renderer: cue export, not timoni](2026-06-16-renderer-cue-export-not-timoni.md)
- [2026-06-14 — Secrets: platform-pull](2026-06-14-secrets-platform-pull.md)
- [2026-06-14 — Pull-based GitOps: timoni + Flux](2026-06-14-pull-based-gitops-timoni-flux.md)
- [2026-06-14 — Platform in-cluster control plane](2026-06-14-platform-in-cluster-control-plane.md)
- [2026-06-14 — Monorepo and Svelte UI](2026-06-14-monorepo-and-svelte-ui.md)
- [2026-06-14 — Identity and linked accounts](2026-06-14-identity-and-linked-accounts.md)

Keep this list in sync when adding a decision.
