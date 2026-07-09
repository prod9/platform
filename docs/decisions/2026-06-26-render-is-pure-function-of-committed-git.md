# Render is a pure function of committed git — the image is a committed literal

- **Date:** 2026-06-26
- **PR:** manual
- **Status:** accepted

## Decision

`ops render` is a pure function of the committed infra repo: it takes the repo's `apps/` sources
(each a render output, importing the shared `defaults/` package) plus the committed `[ops.vars]`
table and produces the `k8s/` manifest tree. Nothing about the target image, or any other deploy
input, is supplied at render time by a CLI flag.

The image ref a workload runs is a **committed CUE literal** in its app (e.g.
`#image: "ghcr.io/prod9/platform:<tag>"`). A new desired state is a git commit that changes that
literal — not a `render --image=…` invocation. `platform publish` builds + pushes the image for a
version; the **operator hand-commits** its ref into the infra CUE (platform never rewrites the
operator's source). The record of what runs is that git commit — pin a `tag@sha256` digest to
dodge stale node cache, but immutability is a tactic, not the invariant.

`[ops.vars]` feed two render routes from one table: CUE `@tag(name)` holes and `.platform`
directive `\(var)` interpolation. The export step injects a var as a CUE tag **only when a
`@tag(name)` actually declares it** — vars destined for directives are not injected (CUE rejects
a tag nothing declares: "no tag for X").

## Rationale

Earlier slices injected the image at render time — `render --inject image=…`, threaded as
`RenderOptions.Image` into `load.Config.Tags` against an `@tag(image)` hole. That made render
impure: the same commit rendered different manifests depending on a CLI argument, so what a
cluster ran was not recoverable from git alone. The image-in-git model restores the GitOps
invariant — the committed repo fully determines the rendered output, and the OCI artifact Flux
reconciles is a deterministic build of a specific commit.

The declared-tag filter is the mechanism that lets one `[ops.vars]` table serve both routes. A
repo whose `apps/` mix CUE apps and `.platform` directives carries vars for both; injecting the
directive-only ones as CUE tags fails the whole export. Discovering the declared `@tag` set (an
AST pass over the loaded apps) and injecting only that subset keeps the shared-table convention
working.

## Consequences

- **Supersedes** the image-injection prose in the
  [linked-CUE-engine ADR](2026-06-23-render-via-linked-cue-engine.md) and the Slice-1 / D3b-3
  passages of the [implementation plan](../scratch/2026-06-16-platformv2-implementation-plan.md):
  there is no `--inject image=` and no `RenderOptions.Image`.
- Committing desired state is `git commit` + `ops publish` + Flux reconcile (or, no-server,
  `ops render` + `kubectl apply`) — never a render-time flag. There is no `deploy` verb.
- `exportCue` probes the apps for declared `@tag` names before injecting; an undeclared
  `[ops.vars]` entry is silently directive-only, not an export error.
