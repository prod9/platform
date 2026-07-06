# ADR: Pull-based GitOps via timoni + Flux + OCI

- **Status:** Revised
- **Date:** 2026-06-14
- **From:** 2026-06 platformv2 design walk

> **Revised 2026-06-16:** the *renderer* is no longer timoni — see
> [renderer ADR](2026-06-16-renderer-cue-export-not-timoni.md) (`cue export` + a manifest
> patch DSL). Read `timoni build` as `cue export` and "timoni module" as "infra CUE
> module" throughout the body below. Everything else in this ADR — pull-based delivery,
> Flux (`OCIRepository` + kustomize-controller), the moving config tag (git history is the
> record), no inbound cluster creds, Keel retirement — **stands**. The render+publish
> mechanics landed 2026-06-17 as `platform ops render` / `ops publish` (`cue export` +
> oras-go).

## Context

The old delivery used Keel tag-watching plus ad-hoc glue. We want full desired-state
reconciliation, CUE-native config, and — load-bearing — **no cluster credentials outside
the cluster**. An earlier draft assumed ArgoCD; we switched to Flux.

## Decision

Delivery is pull-based: **CI renders CUE (`timoni build`) → pushes the rendered manifests
as an OCI artifact → Flux (`OCIRepository` + kustomize-controller) pulls and reconciles.**
The config artifact uses a **moving** tag (its git history is the record); app image refs
inside are committed CUE literals — pin a `tag@sha256` digest to dodge stale cache, but the
record is the commit, not immutability. Multi-env is infra-repo CUE + k8s namespacing, not
per-env tags. CI never holds a kubeconfig. Keel is retired.

## Alternatives rejected

- **ArgoCD** — brought a UI/SSO/Dex stack we'd lean on; we instead surface status in
  platform's own UI and keep the control plane ours. Switched to Flux.
- **Native timoni-Flux controller** — does not exist (timoni's author confirms it's only
  planned post CUE-API stabilization). timoni is used as the CUE renderer in CI; the
  artifact holds rendered manifests, not the raw module.
- **Helm** — banned (magic; not human-traceable). **CI-to-cluster apply** — breaks the
  no-inbound-creds invariant.

## Consequences

- The cluster pulls everything; platform/CI never push in. Scaling = add/upgrade nodes.
- Rollback is deterministic (revert the commit — desired state is git-recorded); drift is
  auto-corrected.
- The Dagger engine and all baseline components install via CUE/timoni manifests, not Helm
  charts.
