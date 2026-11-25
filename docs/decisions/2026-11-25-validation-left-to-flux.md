# `render` does not validate against Kubernetes schemas; Flux's apply does

- **Date:** 2026-11-25
- **PR:** manual
- **Status:** accepted

Supersedes [render validates every document against its Kubernetes schema](2026-11-25-schema-validation-in-render.md).
Current contract: [Lua render](../spec/lua-render.md), "No schema validation".

## Decision

`render` checks no document against a Kubernetes schema. A manifest the cluster would
reject is caught when Flux applies it: the API server refuses it, the `Kustomization`
reports `Ready=False` with the error, and it retries until a fixed tree is published.
Mistakes are kept unlikely at the source, by `infra-lua`'s functions building correct
resources and checking their inputs. platform carries no schema files, `infra-lua`
carries no Kubernetes version, and `platform.toml` gains no cluster setting.

chakrit, 2026-11-25: "let's drop this , mark it down as we're relying on this flux
behavior to protect us from bad schema and the cost is unlikely if we do the infra-lua
funcitons correctly", and on the reason: "there's no simple way to bake validation into
platform itself and having infra-lua carrying a version is also bad because we don't know
which cluster we're deploying to so for now we'll rely on flux because it'd be live in a
real cluster and adding a version to platform.toml is not worth it because it'd be of
limited use and adding only for validation catching typos just doesn't make sense".

## Rationale

**The schema has a version the document does not name.** A document's `apiVersion`
(`apps/v1`) picks a schema, but Kubernetes publishes a separate copy of that schema in
every release, and later copies add fields. Validating means choosing a release.

**Every place to pin that release is wrong or costly.**

- In platform or in the `infra-lua` rock, the release follows a version that has nothing
  to do with the cluster a repo deploys to; downstream repos run different versions of
  both.
- From the cluster itself (its `/openapi/v3`), render needs to know which cluster a repo
  targets, which is a new `platform.toml` setting, plus credentials every renderer would
  need, tenants included.
- Publishing the cluster's schemas somewhere for render to pull only moves the pin.

**The cluster is already the exact validator.** Flux applies to a live cluster whose API
server knows its own version, its CRDs and their CEL rules. A bad manifest costs a red
`Kustomization` and a late discovery, not a broken cluster. What render-side validation
would add is catching typos earlier, which does not justify a setting in every repo.

The validation spike (`infra-defs/spike/validate/FINDINGS.md`, local to `ga42`) showed
`google/jsonschema-go` can read Kubernetes' schemas; that is no longer needed.
