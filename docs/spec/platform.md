# platformv2 — Spec

Consolidates: CUE manifest-gen, CUE framework, infra CLI, this `platform` tool,
keel + argo glue. Codifies four years of accumulated way-of-work into one tool.

## Resume hint

Fresh session: start by dumping the **way of work** narrative below — end-to-end,
from a new repo landing to running in prod. Verbs and artifacts, no tool names.
Contracts fall out of the narrative; don't write them first.

## Way of work

> _Pending._

## Component contracts

> _Pending narrative._

## Server scope

> _Pending narrative._ Credential broker + RBAC is the minimum justified by
> "needs root creds + RBAC." Orchestrator scope (manages k8s, links GitHub /
> Google, triggers Argo) is not yet justified.

## Anchors from prior discussion

- **Language: Go.** CUE has a first-class Go evaluator (`cuelang.org/go`); from
  Rust it's shell-out or WASM. CUE framework is load-bearing → Go.
- **Trigger Argo, don't be Argo.** Default to calling Argo's API over building
  a reconciler unless a concrete need forces it.
- **Skills-first forcing function (optional, cheap).** Writing one coherent
  skill set against the *current* five tools validates the mental model before
  any rewrite — but the user has 4y operational clarity, so this is optional.
- **Lua embed deferred.** Considered for scriptable builders; parked until the
  builder contract is specced. Don't pick the embed before the contract.
- **Sequencing, not big-bang.** Find the spine (CUE-gen → build → version →
  rollout — where four of five tools converge) and consolidate along it.

## Open questions

-
