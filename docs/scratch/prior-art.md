<!-- not spec/decision because: collapsed digest of superseded design scratch; an
     evidentiary record frozen decisions cite, not current design -->

# Prior-Art Digest

Superseded design and planning scratch, collapsed here on 2026-07-12 to kill per-file
cruft while keeping the evidentiary record that frozen decisions cite. Each section below
is the digest of one former scratch file. **Nothing here is current design** — every
durable point it raised now lives in `../spec/` or `../decisions/`, named inline; read
those for truth. The originals were deleted; their inbound citations repoint to the
matching section here.

## platformv2 implementation plan (2026-06-16)

The confirmed 2026-06-16 plan for platformv2: a spine-first, incremental rework building
the delivery pipeline (build → render → publish → reconcile) before the RBAC control
plane. Sequenced Slice 1 (render+publish via `cue export` → Flux OCI artifact), the
manifest-patch DSL slices D1–D3 (hermetic YAML editor + embedded appliance baseline +
bootstrap-writes-DSL), then Slice 2 (Flux install + Keel/ArgoCD cutover) and later server
phases. Landed through D3b-4 and took platform live on stage9. Superseded from 2026-07-06
on: the `../infra` conversion was abandoned (platform self-delivers from a standalone
`./infra`), the `deploy` verb + platform-managed environments were removed, and the
embedded baseline dissolved into the Infra framework. Every durable point is captured —
renderer-cue-export, render-pure-function, render-routes-by-extension, the DSL, generic
vars, dagger-engine-STS, split-log-channels, delivery-verbs-orthogonal, publish-plain-image
— each in its own ADR under `../decisions/` (and `../spec/`). Pure prior-art.

## Slice 1 open questions (2026-06-17)

Ten questions plus one higher-altitude flag raised during the autonomous publish-half
build, each taking a reversible default: the CLI verb surface, publish-target shape,
registry-cred source, OCI artifact format, smoke-test omission, and the DSL/init forks.
All resolved and documented elsewhere: the `ops` namespace was flattened and the
Flux-native artifact format retired (`../decisions/2026-07-05-infra-publishes-as-plain-image-retire-oras.md`),
verbs made orthogonal (`../decisions/2026-07-05-delivery-verbs-are-orthogonal.md`), generic
vars ruled (`../decisions/2026-06-17-generic-ops-vars-single-config.md`), and the
appliance/embedded-init ruled (`../decisions/2026-06-17-opinionated-appliance-embedded-init.md`).
Pure prior-art.

## D3b-4 baseline design prep (2026-06-19)

AFK design-prep for the cluster baseline, derived from the real `infra` repo: the
foreign-install file set (cert-manager, nginx-gateway + CRDs, flux, argocd-reference as
`baseline/*.platform`), the `settings.toml → [vars]` version-pin migration (interpolated
`\(var)` into `download` URLs), the NGF recipe (plain-manifest downloads, `serverTokens=off`,
firewall-id annotation via a StrategicMerge patch), and the `platform init` write-path with
a generic checkbox/choice picker. Fully superseded: the directive syntax predates the
2026-06-20 DSL redesign, the picker model was reversed by the unconditional-install ruling
(`../decisions/2026-07-11-baseline-dissolves-into-infra-framework.md`), and the CUE engine
became a StatefulSet (`../decisions/2026-06-21-dagger-engine-statefulset-tcp.md`). Durable
facts live in `../spec/scaffolding.md`, `../spec/manifest-patch-dsl.md`, and
`../spec/config-allocation.md`. Pure prior-art.

## Builders reshape design pass #4 (2026-06-29)

Design pass on the old `builder/` package: it mapped the structural gaps (copy-paste
run-stage, scattered cmd/args/outdir resolution, per-builder drift, dead `Class()`
metadata, the order-sensitive registry) and resolved the strategic fork — infra-render
became a **new builder returning a plain `*dagger.Container`** (contract unchanged), with
Flux compatibility moved producer→consumer via `layerSelector`. That "builder" concept is
exactly what shipped, renamed the **Framework** model. Every durable conclusion is now in
`../spec/frameworks.md` and the 2026-07-05 / 2026-07-11 decisions
(`infra-publishes-as-plain-image`, `test-in-build-is-a-hard-gate`, `platform-fhs-container-layout`,
`baseline-dissolves-into-infra-framework`). The only uncaptured item — a deferred
"one framework per class, language as metadata" rethink — is speculative, not durable.
Pure prior-art.

## Platform-as-CI architecture design (2026-06-29)

The target architecture for turning platform from a local CLI into a self-hosted,
webhook-driven CI/CD server (`platform serve`) that reuses the engine per-request and
delivers via Flux. Pillars: one Go module (`core`/`cli`/`srv`/`webui` as conceptual layers,
server in-binary, no `go.work`, no `core/` umbrella); zero platform RBAC with authz
delegated to GitHub; a GitHub App minting installation vs user-to-server tokens; a repo-prep
phase in `srv` above the builders; a persistent `/var/cache/platform` bare-mirror + worktree
cache (full clones, no `--depth 1`); and a 3-step sequencing (prove CLI path → wrap in `srv`
→ `webui`). Still the intended target (`srv/` not yet built), and now transcribed nearly
1:1 into `../spec/platform-server.md`, with the auth ruling frozen in
`../decisions/2026-06-29-platform-server-github-app-zero-rbac.md`. The one point that had
been captured nowhere — the deliberate rejection of an `api/` contract layer and the hard
rule that `cli` must not import `srv` — was promoted into `../spec/platform-server.md` on
2026-07-12. Now fully absorbed.

## Framework refactor migration plan (2026-07-08)

The implementation tracker for the framework refactor (`builder/` → `framework/`, a
`Framework` as sole owner of a project type: recognizes / scaffolds / builds itself;
`baseline/` and top-level `scaffold/` dissolved into `framework/scaffold` + the Infra
framework). Implemented 2026-07-11; the settled design was promoted to `../spec/frameworks.md`
+ `../spec/scaffolding.md` and ruled in
`../decisions/2026-07-11-baseline-dissolves-into-infra-framework.md`. This file only tracked
migration state. Pure prior-art.

## Terminology pass + spec audit plan (2026-07-11)

The working plan for a two-pass terminology + spec-audit sweep over every durable
instruction surface. Its 11 terminology clusters were all ruled and applied; the rulings
are frozen in `../decisions/2026-07-11-terminology-lexicon.md` and the doc edits landed in
`../spec/`. This file only tracked the in-flight pass. Pure prior-art.
