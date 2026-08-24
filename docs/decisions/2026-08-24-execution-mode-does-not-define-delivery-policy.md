# Execution mode does not define delivery policy

- **Date:** 2026-08-24
- **PR:** manual
- **Status:** accepted

## Decision

Platform has two peer execution modes over shared build/publish capabilities: an
operator-invoked local CLI and an asynchronous CI/CD server formed by `srv` plus the
worker. Execution mode answers who invokes an operation, from where, and when; delivery
policy separately answers which successful builds publish.

The CI/CD server builds every non-deleted push. App repositories publish successful tag
builds under the exact tag, with no naming restriction. Infra repositories publish every
successful build under `latest`. How the server identifies the repository kind remains
unresolved.

The opinionated app/infra workflow is a convention over both execution modes. This
repository's own release runbook is a fourth, narrower concern and imposes no capability
or naming restriction on consuming repositories.

The current contract is specified in
[`../spec/execution-modes.md`](../spec/execution-modes.md).

## Rationale

The earlier one-engine/two-drivers ruling correctly separated `release` from
`publish`, but described the future server as a version-tag watcher. That imported
local release strategy into server CI and made “what builds?” indistinguishable from
“what publishes?” It also made this repository's `v0.9.x` release procedure look like
a product constraint.

The shared engine explains why the modes are related, not why they run. Naming the driver,
publish policy, and repository-specific runbook as separate layers prevents a rule from
one layer becoming an accidental filter in another.

## Supersession

This decision supersedes only the server tag-watch and automatic-publish cadence in the
[delivery-verbs decision]. That decision's release/publish orthogonality, absence of a
deploy verb, and one shared publish engine with local and server drivers remain accepted.

[delivery-verbs decision]: 2026-07-05-delivery-verbs-are-orthogonal.md
