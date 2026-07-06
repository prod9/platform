# ADR: Platform as an in-cluster control plane (single home cluster)

- **Status:** Accepted
- **Date:** 2026-06-14
- **From:** 2026-06 platformv2 design walk

## Context

Platform needs to mint kube tokens, read reconciler status, and serve secrets — all of which
naively imply holding a cluster credential. That would violate "no cluster creds outside the
cluster." We also had to decide single- vs multi-cluster scope for v2.

## Decision

Platform runs **inside** the home cluster and acts through its **pod ServiceAccount** — so
its "cluster credential" is in-cluster (projected, auto-rotated), never external. v2 targets
a **single home cluster**. The pod SA is the "root SA"; it mints short-lived, RBAC-scoped
per-project SA tokens via `TokenRequest` for `kubectl` access (no API-server OIDC config, no
cloud IAM).

## Alternatives rejected

- **kube-API-server OIDC** (platform as issuer) — clean UX but per-cluster control-plane
  config + per-cloud IAM friction. Rejected.
- **Separate in-cluster token-broker service** — collapses into "platform itself" once
  platform is in-cluster.
- **External platform holding a cluster cred** — breaks the invariant.

## Consequences

- Multi-cluster (central control-plane + per-cluster agents, pulling outbound) is **phase
  2**; the agent generalizes the token-broker.
- Per-environment isolation is a namespace in the home cluster — expressed in the infra repo's
  CUE (a template instantiated per env), not a platform-managed target.
- First-cluster bootstrap is a manual seed (Flux + `platform-init`, which includes platform
  itself).
