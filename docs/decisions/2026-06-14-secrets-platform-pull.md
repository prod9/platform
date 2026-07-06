# ADR: Workload secrets via platform-pull init-container

- **Status:** Accepted
- **Date:** 2026-06-14
- **From:** 2026-06 platformv2 design walk

## Context

We had no secrets system. Constraints: no plaintext/decrypt-key sitting in the reconciler;
the plan's non-goals (no custom controller/CRD); and no inbound cluster creds. SOPS-in-git
and ESO were considered.

## Decision

Secret **values** live in platform (Postgres, encrypted at rest, RBAC'd, audited). The
`infra/` CUE templates a per-workload **init-container** that **pulls** the workload's
secrets from platform's API at pod start (outbound, authed by the pod identity) and writes
them for the main container. Git holds only references.

## Alternatives rejected

- **SOPS + KSOPS in the reconciler** — puts the decrypt key + rendered plaintext in the
  reconciler; opaque blobs in git.
- **External Secrets Operator** — a controller + CRDs (plan non-goal); heavier than
  needed.
- **Platform pushes the Secret at apply time** — requires inbound cluster creds; rejected.

## Consequences

- Rotation/revocation is central (platform owns values); no key in the reconciler.
- The materialized in-pod secret is still standard k8s plaintext at rest in etcd —
  secrets-store-CSI is a later, stricter option.
- The init-container pulls from the local in-cluster platform (no external egress).
