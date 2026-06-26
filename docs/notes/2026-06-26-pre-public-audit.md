# Pre-public audit — prod9/platform (2026-06-26)

**Decision: stay PRIVATE for now. No scrubbing performed.** This note records the audit so a
future "make it public" doesn't redo the work — it lists exactly what to do at that point.

## Method

Five parallel read-only audit agents over the working tree + full git history (369 commits,
all branches): secrets, embedded assets, Go source, deps/build/license, docs sensitivity.

## Findings

| Dimension          | Verdict                                                              |
| ------------------ | ------------------------------------------------------------------- |
| Secrets (history)  | CLEAN — no tokens/keys/creds in any commit; registry auth env-only  |
| Go source          | SHIP — no embedded secrets; vanity carries none                     |
| Deps / build       | OK — outsider can `go build`; `fx.prodigy9.co` public via proxy; no `replace` |
| Embedded assets    | 1 SCRUB — real Linode firewall ID `11222746` in `DefaultVars`       |
| Docs sensitivity   | MODERATE SCRUB — internal planning / cluster + customer names       |

No hard credential blockers. Nothing was scrubbed (repo staying private).

## To do BEFORE flipping public (deferred)

1. **LICENSE** — none exists → defaults to all-rights-reserved. Pick one (MIT/Apache-2.0) or
   knowingly keep source-available.
2. **Linode firewall ID** — `core/baseline/embed.go:38` ships `NGINX_GATEWAY_FIREWALL_ID =
   "11222746"` (a real prod9 Linode resource) inside the binary, echoed in docs/tests. Swap to
   an operator-supplied placeholder.
3. **Docs scrub** — internal-strategy disclosures (not secrets): CLAUDE.md rework banner +
   fleet/customer names (`naxon`, `fi`), `PLANS.md` roadmap, `docs/notes/*` (cluster topology,
   stage9/prodigy9, deployment state), the school-handoff note. Recommended: exclude
   `docs/notes/` from the public repo wholesale; trim CLAUDE.md to public-facing conventions.
