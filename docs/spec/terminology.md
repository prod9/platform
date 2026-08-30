# Terminology

Status: **draft.** These terms keep one meaning across configuration, code, CLI, web UI,
and documentation. Historical renames and their rationale remain in the
[terminology decision](../decisions/2026-07-11-terminology-lexicon.md).

## Product and execution

- **platform** — the product. Architecture is named **arch**, including `local_arch`,
  `publish_arch`, and `BuildUnit.Arch`.
- **engine** — the reusable source and build driver around Dagger. The CUE dependency is
  the **linked CUE evaluator**, not an engine.
- **framework** — one stack-specific project implementation. A module selects it with
  `framework = "..."`; `builder` is a deprecated input alias, not current vocabulary.
- **component** — a render-able entry under `apps/` and its corresponding manifest output.
- **layout** — a workspace shape. Runtime-shape families are descriptive categories, not
  a `Class` contract.

## Verbs and artifacts

- **scaffold** / **scaffolding** — create the source files a project starts from.
- **render** — transform the infra repo's CUE and `.platform` sources into a manifest
  tree.
- **export** — write a built image tarball. **CUE export** is CUE's qualified tool
  vocabulary and does not rename the platform command.
- **release** — cut a version marker. **publish** — build and push an image. Neither verb
  implies the other.
- **deploy** — not a platform verb. Deployment is the operator committing desired state
  in the infra repo, followed by infra publication or direct application of a render.
- **launcher** — the repository's version-pinned `./platform` script.

## Values and controls

- **strategy** — the release-naming strategy: `semver`, `datestamp`, `timestamp`, or
  `rolling`.
- **rolling** — the non-versioned strategy whose emitted name and registry tag are
  `latest`. `rolling` names the strategy; `latest` names its value.
- **bump** — the semver increment selected by `BumpAny`, `BumpPatch`, `BumpMinor`, or
  `BumpMajor`. It is not a component.
- **defaults** — a site-local term. The `defaults/` CUE package contains shared infra
  definitions; Go defaults are parser-owned config values.
- **`--force`** — override a safety refusal: release accepts a dirty worktree; init
  replaces existing files. **`ALWAYS_YES=1`** answers yes/no confirmation prompts. Neither
  substitutes for the other.
