# `ops render` routes CUE and `.platform` by extension; one render, no committed YAML

- **Status:** accepted
- **Date:** 2026-06-18
- **From:** D3b-3 design discussion (chakrit)

## Context

Mid-D3b the delivery path was briefly split into two activities: a separate run-the-DSL
command that fetches/patches foreign installs and **vendors** the manifests into the infra
repo (model II), versus `ops render` (`cue export`) for the authored manifests. That framing
was committed earlier today (`9274fa3`, and the D3b-3 doc bullets).

Two facts reopened it:

- The `.platform` extension makes a single `ops render` able to **dispatch by input type** —
  there is nothing to split.
- The real infra CUE (`../infra/apps/*.cue`) is authored as **filename → document-list**
  maps (`"gateway.yaml": gw`, `"cluster-issuer.yaml": [issuer]`), so `cue export` already
  yields *named files* — the same output contract as the DSL's `emit`. The "stream vs files"
  mismatch that motivated the split was an artifact of Slice-1's testbed
  (`cue export -e objects --out yaml` → one stream), not the real layout.

The [appliance ADR](2026-06-17-opinionated-appliance-embedded-init.md) (decision 4) already
framed the DSL `emit` as "fed through the Slice 1 render/publish path." Model II was the
detour; this restores that.

## Decision

1. **One `ops render`, routed by extension.** It walks the infra repo: `.cue` → file-map
   `cue export`; `.platform` → assembly (`core/baseline.Select` over `[ops.vars]`) →
   `dsl.Apply` (download upstream → patch → `emit`). Both write **named files** into a
   `k8s/<component>/` render-output tree.

2. **Model I — render-time, nothing rendered is committed.** The DSL runs at render time.
   The infra repo holds *sources* (`apps/*.cue` + `baseline/*.platform`); the `k8s/` tree is
   render output, shipped by `ops publish`. This keeps the no-committed-rendered-YAML rule
   intact for the whole pipeline.

3. **Uniform output contract:** filename → document-list → named files. `core/baseline` owns
   the directive-file → `k8s/<component>` directory mapping, where `<component>` is the
   filename stem before any `@variant` / `+flag` marker (so all variants and overlays of one
   component co-locate).

4. **No separate run-DSL command.** `ops run` survives only as an optional dev convenience
   (render just the foreign bits to disk); it is not on the critical path.

## Consequences

- **Slice-1 render/publish is reworked.** `core/gitops.Render` moves from the flat
  `-e objects` single stream to a filename→docs **file-map** emitter (writes
  `k8s/<component>/*.yaml`); `ops publish` packages the resulting tree. This revises
  already-landed Slice-1 code.
- **Open #7 is now literally satisfied** — `ops render` reads (and runs) the directives from
  the infra repo; edits to directive files or `[ops.vars]` need no recompile.
- **Supersedes the interim model-II framing** committed earlier today (`9274fa3` and the
  D3b-3 bullets in the spec/roadmap); those docs are re-pointed to this decision.
- **Cost:** foreign manifests are re-downloaded each render until the deferred `download`
  cache/checksum lands. Acceptable: `ops render` already pulls infra-defs from ghcr
  (`CUE_REGISTRY`), so render is not offline today, and there are no mission-critical
  workloads.
- The `.platform` extension and whole-file gating (option C) are unchanged.
