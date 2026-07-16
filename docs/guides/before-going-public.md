# Before going public

The repo is private by ruling — see
[repo-stays-private](../decisions/2026-06-26-repo-stays-private.md) for why, and the audit
that backs this list (clean on secrets; these are the non-secret readiness gaps). Work
this checklist before flipping the repo public; none is a credential blocker, each is
deliberate exposure work.

## 1. Add a LICENSE

No LICENSE exists, so the repo defaults to all-rights-reserved. Pick one — MIT or
Apache-2.0 — or knowingly keep it source-available. Add the file at the repo root before
flipping.

## 2. Placeholder the Linode firewall ID

`framework/infrabase.go` ships `NGINX_GATEWAY_FIREWALL_ID = "11222746"` in `DefaultVars` — a
real prod9 Linode resource, baked into the binary and echoed in docs and tests. Swap it
for an operator-supplied placeholder. Sweep docs and tests for the literal ID and update
those too.

## 3. Exclude docs/scratch from the published tree

`docs/scratch/*` carries internal planning not meant for outsiders — cluster topology, the
stage9/prodigy9 setups, deployment state, the school-handoff note. Exclude `docs/scratch/`
wholesale from the public repo rather than scrubbing note by note.

## 4. Scrub internal-strategy disclosures in docs

Non-secret internal posture leaks through the remaining docs. Trim before publishing:

- `CLAUDE.md` — drop the rework banner and fleet/customer names (`naxon`, `fi`); keep only
  public-facing conventions.
- `PLANS.md` — internal roadmap; remove or trim.

## 5. Re-verify history is clean

The 2026-06-26 audit found no secrets across 369 commits and all branches. If history has
grown since, re-scan for tokens/keys/creds before flipping — registry auth must stay
env-only, never committed.
