# Decisions Log

**Point-in-time defenses against future re-litigation** — rulings made on
a specific date for a specific question, recorded so the same argument
doesn't have to be re-fought next quarter. Each entry is frozen at the
moment of decision; if a later ruling reverses it, write a new dated
decision that links back and mark the old one `superseded`.

**Spec first — always.** Never write a decision here before updating [`../spec/`](../spec/)
to the design it rules on. The spec is the source of truth for current state; this log is
only the frozen *why*. A decision recorded while the spec still teaches the old design gets
re-litigated — the failure this whole discipline exists to prevent. See
[Spec-first](../README.md#spec-first--the-spec-is-the-most-important-document).

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
exploratory write-up — that's scratch, not a decision. Use `../scratch/`. If
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

- [2026-07-16 — The scaffolded baseline is provider-neutral; cloud wiring is the infra repo's edit](2026-07-16-baseline-is-provider-neutral.md)
- [2026-07-16 — The v0.9 line is platformv2; v2 cuts are patch releases](2026-07-16-v0.9-line-is-platformv2.md)
- [2026-07-12 — CUE module path is a scaffold input read from cue.mod, not the repository](2026-07-12-cue-module-path-is-a-scaffold-input.md)
- [2026-07-11 — Terminology lexicon: one word, one concept](2026-07-11-terminology-lexicon.md)
- [2026-07-11 — Baseline installs unconditionally; dissolves into the Infra framework](2026-07-11-baseline-dissolves-into-infra-framework.md)
- [2026-07-05 — Infra publishes as a plain Dagger image; retire oras-go](2026-07-05-infra-publishes-as-plain-image-retire-oras.md)
- [2026-07-05 — Delivery verbs are orthogonal; one publish engine, two drivers](2026-07-05-delivery-verbs-are-orthogonal.md) *(read `ops publish`/`ops render` as `publish`/`render`)*
- [2026-07-05 — Test-in-build is a hard gate; blackbox-first testing](2026-07-05-test-in-build-is-a-hard-gate.md)
- [2026-07-05 — Platform FHS container layout; cmd is the runtime command](2026-07-05-platform-fhs-container-layout.md)
- [2026-06-29 — Platform server: GitHub App, zero platform RBAC](2026-06-29-platform-server-github-app-zero-rbac.md) *(revised 2026-07-18: stress-tested against observability, holds)*
- [2026-06-26 — Render is a pure function of committed git](2026-06-26-render-is-pure-function-of-committed-git.md)
- [2026-06-26 — Repo stays private for now](2026-06-26-repo-stays-private.md)
- [2026-06-24 — Split build and server log channels](2026-06-24-split-build-and-server-log-channels.md)
- [2026-06-23 — Render via the linked CUE engine](2026-06-23-render-via-linked-cue-engine.md)
- [2026-06-22 — Flat baseline, install-time selection](2026-06-22-flat-baseline-install-time-selection.md) *(picker half superseded by 2026-07-11)*
- [2026-06-21 — Dagger engine: StatefulSet + TCP](2026-06-21-dagger-engine-statefulset-tcp.md)
- [2026-06-20 — DSL: focus scope, strict values](2026-06-20-dsl-focus-scope-strict-values.md)
- [2026-06-18 — Render routes .cue and .platform by extension](2026-06-18-render-routes-cue-and-platform-by-extension.md) *(rulings stand; `baseline.Select`/marker-grammar mechanics superseded)*
- [2026-06-17 — Opinionated appliance, embedded init](2026-06-17-opinionated-appliance-embedded-init.md) *(rulings stand; `bootstrap`/`bootstrapper` mechanics superseded)*
- [2026-06-17 — Generic ops vars, single config](2026-06-17-generic-ops-vars-single-config.md)
- [2026-06-16 — Renderer: cue export, not timoni](2026-06-16-renderer-cue-export-not-timoni.md)
- [2026-06-14 — Secrets: platform-pull](2026-06-14-secrets-platform-pull.md)
- [2026-06-14 — Pull-based GitOps: timoni + Flux](2026-06-14-pull-based-gitops-timoni-flux.md)
- [2026-06-14 — Platform in-cluster control plane](2026-06-14-platform-in-cluster-control-plane.md)
- [2026-06-14 — Monorepo and Svelte UI](2026-06-14-monorepo-and-svelte-ui.md)
- [2026-06-14 — Identity and linked accounts](2026-06-14-identity-and-linked-accounts.md)

Keep this list in sync when adding a decision.
