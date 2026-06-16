# Renderer is `cue export` + a directive patch engine, not timoni

- **Date:** 2026-06-16
- **PR:** manual
- **Status:** accepted
- **Supersedes:** the *renderer* choice in
  [2026-06-14-pull-based-gitops-timoni-flux](2026-06-14-pull-based-gitops-timoni-flux.md).
  That ADR's Flux + OCI + pull-based decision stands; only "render via `timoni build` " is
  reversed here.

## Decision

Two renderers for two distinct jobs; Flux still applies both via the unchanged OCI +
`OCIRepository` path:

- **Manifests we author** — `cue export` over infra-defs-based modules, plus a small Go
  step that emits the resulting object list as a multi-doc (`---`) YAML stream. CUE owns
  values, schema, and composition.
- **Third-party manifests we adapt** — a line-oriented directive patch engine (folded from
  infra-cli's `pipelines` + `yamleditor`), spec'd in
  [`../spec/manifest-patch-dsl.md`](../spec/manifest-patch-dsl.md). Go fetches and pins
  the upstream manifest; the directives patch it by name; CUE supplies the values.

timoni is dropped from the design.

## Rationale

The earlier ADR had already collapsed timoni's role to "the CUE renderer in CI" — Flux's
kustomize-controller consumes *rendered manifests*, never the timoni module, and no native
timoni-Flux controller exists. So timoni was load-bearing for exactly one verb: CUE → a
set of k8s manifests. Three facts undercut even that:

- **infra-defs already is the packaging layer.** `prodigy9.co/defs` (thin k8s wrappers +
  `parts/` mixins + `packs/` app blueprints, distributed as a CUE OCI module) fills the
  module/values/parameterization role a timoni module would. A timoni layer on top is a
  *second* packaging system — the "knowledge scattered across tools" v2 exists to kill.
- **timoni isn't on the host; `cue` is.** Rendering via timoni forced a timoni-in-Dagger
  bootstrap and vendored k8s schemas (`cue.mod/gen`) committed into every fixture — bloat
  for zero gain, since Flux applies the output regardless of what rendered it.
- **The hard case isn't render, it's patching foreign manifests.** Adapting upstream
  installs (cert-manager, NGF) means targeted, by-name, append-to-a-varying-list edits
  that neither CUE unification, nor timoni (CUE underneath), nor kustomize expresses
  cleanly. That needs an imperative patch engine, which timoni does not provide.

Why not the obvious alternatives:

- **timoni** — adds a second packaging layer over infra-defs, a host-absence/Dagger
  bootstrap, and vendored-schema bloat, while doing nothing load-bearing.
- **Embedded scripting (Lua/Starlark/CEL/yq) for patching** — a general-purpose script
  can't be bounded by reading it (the Helm/TypeScript failure mode); a closed directive
  vocabulary can. Rejected for the same reason Helm is banned.
- **Pure CUE for foreign-manifest patching** — unification intersects, it cannot append to
  a concrete list; the workaround is hand-rebuilding the object tree with copy-all-except
  comprehensions: unreadable, and it inverts CUE's strengths.

## Consequences

- One toolchain: Go + CUE (+ infra-defs), no timoni. Matches the single-streamlined-tool
  goal.
- Render needs no Dagger (cue is on host) and no vendored k8s schemas; fixtures are thin
  `prodigy9.co/defs` consumers.
- infra-cli folds in by *shrinking*: N per-component generator subcommands become one
  generic fetch+patch driver + per-component directives and CUE data.
- Slice 1 of the implementation plan is reworked accordingly (no timoni-in-Dagger).
