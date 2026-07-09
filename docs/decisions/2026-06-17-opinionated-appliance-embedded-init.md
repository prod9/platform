# Platform is an opinionated appliance; the baseline is embedded, shipped as an init DSL

- **Status:** accepted
- **Date:** 2026-06-17
- **From:** Slice 1 close-out discussion (chakrit)

## Context

The earlier design (`config-allocation.md`, `platform.md`) modelled platform as a
config-externalising GitOps engine: the cluster baseline lived in a separate, user-owned
`platform-init` **repo** (CUE, git), and platform was "just the build tool." That framing
is wrong about what platform is.

Platform is **not generic**. It is strongly opinionated and tied to one specific cluster
setup — Flux + cert-manager + NGINX Gateway Fabric + the Dagger engine + a specific
Gateway-API topology — and it does not work against any other. The baseline is therefore
**platform's opinion**, not the operator's configuration, and must be version-locked with
the tool that depends on it.

The repo already proves the pattern: `bootstrapper/` `go:embed`s its templates
(`platform.template`, `buildkite.pipeline.yaml.template`) and writes them into target
repos. The cluster baseline is the same move, one layer down.

## Decision

1. **Opinionated appliance.** Platform ships *the* setup. The cluster baseline is embedded
   in the tool and version-locked with it — not external configuration the operator
   maintains.

2. **The baseline is embedded, not a separate repo.** This supersedes the
   "`platform-init` repo" rows in `config-allocation.md` and `platform.md`. There is no
   hand-maintained sibling repo; the baseline source lives in this repo and is **emitted**
   by platform.

3. **The baseline is shipped as an init DSL package.** Foreign installs (Flux,
   cert-manager, NGF, engine) are expressed as [manifest patch DSL](../spec/manifest-patch-dsl.md)
   directives — `download` upstream, patch by name, `emit`. CUE stays the form for
   manifests we author (namespaces, RBAC, Gateway, the platform Deployment); the DSL is
   for the foreign ones we don't.

4. **Bootstrap writes the DSL into the infra repo.** Rather than platform rendering and
   seeding directly, bootstrap drops the init directive files into the infra repo, where
   they are applied manually or fed through the Slice 1 render/publish path (the DSL's
   `emit` tail). Same write-once-then-operator-owns shape as `bootstrapper/`.

## Consequences

- **The DSL is pulled forward.** It was Phase C; it becomes the next build, because the
  init package (3) and bootstrap (4) both depend on it. See the
  [roadmap](../scratch/2026-06-16-platformv2-implementation-plan.md).
- **The no-committed-rendered-YAML rule is unaffected.** That rule
  (`config-allocation.md`) governs the *downstream reconcile loop* — per-target desired
  state → OCI → Flux. The baseline/bootstrap is the cold start *outside* that loop; a
  pinned Flux seed (Flux ships YAML, not CUE, and its lifecycle is never self-managed) is
  legitimate there.
- **Dogfood target is the real `infra` repo**, which already carries
  `apps/cert-manager.cue`, `apps/gateway.cue`, and `k8s/{cert-manager,nginx-gateway,…}` —
  the exact foreign installs the DSL adapts.
- **The port is bounded.** The DSL backend is a port of `infra-cli/pipelines/*` +
  `pipelines/yamleditor` (~676 LOC incl. tests); only the directive parser and the
  field-select path form are new code.
- **Baseline version-bump sync is merge** (resolved 2026-06-18, open #7): re-`bootstrap`
  overwrites the directive files but merges `[ops.vars]` (append new keys, preserve existing
  operator values); `ops render` reads directives from the infra repo, not the embed, so
  edits need no recompile. Bootstrap prints an analysis plan and confirms before writing;
  `--force` applies unprompted.
- **Open:** `download` checksum-pinning and the CUE/DSL boundary for baseline components —
  tracked in the [slice-1 open questions](../scratch/2026-06-17-slice1-open-questions.md).
