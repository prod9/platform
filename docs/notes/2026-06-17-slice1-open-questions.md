# Slice 1 (publish) — open questions for batch review

**Status:** parked for chakrit's review · raised during the autonomous publish-half build
(2026-06-17). None block the slice; each took a reasoned default, recorded here so the
choice is visible and reversible.

## Decisions taken (confirm or override)

1. **Command surface — `ops` namespace.** `render` moved under `platform ops render`;
   publish lands as `platform ops publish`. The old top-level container-release `publish`
   is untouched. *Default reason:* sidesteps the name collision; groups the delivery
   spine; leaves room for Slice 2 verbs (reconcile/cutover).

2. **~~Publish target flag shape~~ — superseded 2026-06-17 (chakrit).** `--to` is gone.
   The target is convention-over-configuration: an `[ops]` section in `platform.toml`
   (`image`, `tag`), where `image` is **inferred from `repository`** (`github.com/x` →
   `ghcr.io/x`, the same rule as `ImageName`) and `tag` defaults to `latest`. So
   `platform ops publish` needs no target flag; `--tag <env>` overrides the moving tag for
   a per-env publish. *Dependency:* this presumes a `platform.toml` in the infra repo —
   see fork #9.

3. **Creds source.** gitops-local `REGISTRY` / `REGISTRY_USERNAME` / `REGISTRY_PASSWORD`
   fx config vars, reading the **same env names** as `builder/` but defined independently
   in `core/gitops` (no import of `builder/`). *Reason:* keeps the spine decoupled from
   the legacy package (B1 moves it anyway); same env contract.

4. **Artifact format — Flux-native.** tar+gzip layer (`manifests.yaml`), config media type
   `application/vnd.cncf.flux.config.v1+json`, layer media type
   `application/vnd.cncf.flux.content.v1.tar+gzip`. *Reason:* Slice 2's Flux
   `OCIRepository` consumes it unchanged. Validated by unit round-trip, not yet against a
   live Flux.

5. **No publish smoke test.** A live-registry round-trip needs creds + network, which the
   1m honest-timeout harness forbids. Covered instead by a Go unit round-trip through an
   `oci.Store` filesystem layout. Live push validated manually / in Slice 2.

## Phase A′ (DSL + init) forks — raised 2026-06-17

Settled in discussion, captured in the
[appliance ADR](../decisions/2026-06-17-opinionated-appliance-embedded-init.md):

- **Resolved — `platform-init` is embedded, not a separate repo.** The baseline is
  platform's opinion, version-locked with the tool, shipped as an init DSL package;
  bootstrap writes it into the infra repo. Specs updated.
- **Resolved — DSL pulled forward** to Phase A′ (before Slice 2's reconcile), since the
  baseline depends on it.

Still open (none block D1):

6. **~~`download` reproducibility~~ — DEFERRED 2026-06-18 (chakrit).** The DSL pins
   version in the URL but not a checksum; live-fetch-at-render risks network flakiness and
   "upstream deleted the release." Proposal was a `download URL sha256=…` guard. D2 shipped
   plain `download URL` without it. Revisit together with a body/size cap — `download`'s
   unbounded `io.ReadAll` and `extract`'s uncapped decompression sit on the same
   network+decompression trust boundary, so checksum + size limits are one design pass.
7. **Baseline version-bump sync.** When platform's embedded baseline bumps a pinned
   upstream version, does it **rewrite** the infra repo's written DSL, or is it
   **write-once / operator-owns** after the first bootstrap (the `bootstrapper/` shape)?
   Default leaning: write-once, matching the existing template precedent. Decide at D3.
8. **CUE/DSL boundary for the baseline.** Which baseline components are authored as CUE
   (ours: namespaces, RBAC, Gateway, platform Deployment) vs DSL (foreign: Flux seed,
   cert-manager, NGF, engine). Sketched in the ADR; confirm exact split while authoring D3
   against the real `infra` repo.
9. **`ops publish` presumes a `platform.toml` in the infra repo.** Today neither `infra`
   nor `infra-stage9` has one, so the `[ops]`/`repository` convention has nothing to read
   from. Bootstrap will write it (the `bootstrapper/` pattern, sourcing `repository` from
   the git remote). Deeper convention worth weighing: have `ops publish` infer
   `repository` straight from `git remote get-url` when the toml omits it — zero-config in
   any infra repo. Decide alongside D3/bootstrap.
10. **~~DSL `${var}` value source~~ — RESOLVED 2026-06-17 (chakrit).** Eliminate
    `settings.toml`; one file (`platform.toml`). `[ops.vars]` is a **generic open
    `map[string]string`** — the processor stores it verbatim, no per-software fields, so
    the DSL owns its var vocabulary and adding/removing a `${var}` never touches the Go
    DTO.
    Values are strings (bools too: `experimental = "true"`, interpreted by the assembly
    layer). `[ops].image`/`tag` stay typed (publish target ≠ DSL var). See the
    [generic-ops-vars ADR](../decisions/2026-06-17-generic-ops-vars-single-config.md).
    settings.toml content migrates into `[ops.vars]` at D3.

## Higher-altitude flag (chakrit raised, 2026-06-17)

**Registry creds delivery vs the platform secrets system.** The `REGISTRY*` vars exist
because Buildkite agents pre-load configurable vars into the agent env (creds never sit in
`pipeline.yaml`). That model needs revisiting so it composes cleanly with platform's
planned pull-based secrets system (see
[`docs/decisions/2026-06-14-secrets-platform-pull.md`](../decisions/2026-06-14-secrets-platform-pull.md)).
For now the config-var path is the accepted stopgap. **Not in this slice** — flag for the
secrets-system design pass.
