# Repo stays private for now

Date: 2026-06-26
Status: **accepted**

## The ruling

`prod9/platform` stays **private**. A pre-public audit ran — five parallel read-only
agents over the working tree and full git history (369 commits, all branches): secrets,
embedded assets, Go source, deps/build/license, docs sensitivity — and found **no hard
credential blocker**. Going public is nonetheless deferred: the readiness work it surfaced
is not yet worth doing, so nothing was scrubbed.

## Why not-yet-worth-it

The audit came back clean on the blocking dimensions but flagged non-secret readiness gaps
that a public flip would first have to close:

| Dimension         | Verdict                                                          |
| ----------------- | --------------------------------------------------------------- |
| Secrets (history) | CLEAN — no tokens/keys/creds in any commit; registry auth env-only |
| Go source         | SHIP — no embedded secrets; vanity carries none                 |
| Deps / build      | OK — outsider can `go build`; `fx.prodigy9.co` public via proxy; no `replace` |
| Embedded assets   | 1 SCRUB — real Linode firewall ID in `DefaultVars`              |
| Docs sensitivity  | MODERATE SCRUB — internal planning + cluster/customer names     |

Two soft gaps, neither a secret, both requiring deliberate work before exposure:

- **No LICENSE.** Absent a license the repo defaults to all-rights-reserved. Publishing
  demands a deliberate choice (MIT/Apache-2.0, or knowingly source-available), not a
  default.
- **Internal-strategy disclosure.** The embedded assets carry a real prod9 Linode firewall
  ID, and the docs carry internal planning — the rework banner, fleet/customer names,
  roadmap, and cluster topology in `docs/scratch/`. None are credentials; all are internal
  posture we don't hand out for free.

Nothing here is a leak, so private carries no risk and buys time; public would first cost
the scrub-and-license pass above for a repo with no external consumer yet. Not worth it
now.

## Consequences

- The repo remains private; no scrubbing is performed while it stays private.
- The audit is not re-run on a future public flip — the actionable checklist is captured
  in [before-going-public](../guides/before-going-public.md). Do that work, then flip.
