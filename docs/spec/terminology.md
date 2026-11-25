# Terminology

Status: **draft.** These terms keep one meaning across configuration, code, CLI, and
documentation. Historical renames and their rationale remain in the
[terminology decision](../decisions/2026-07-11-terminology-lexicon.md).

## Product and execution

- **platform** — the product, a command-line tool. Architecture is named **arch**,
  including `local_arch`, `publish_arch`, and `BuildUnit.Arch`.
- **engine** — the reusable build driver around Dagger. Nothing else is an engine:
  `render` runs Lua on the embedded **Lua VM**.
- **framework** — one stack-specific project implementation. A module selects it with
  `framework = "..."`; `builder` is a deprecated input alias, not current vocabulary.
- **component** — a render-able entry under `apps/` and its corresponding manifest output.
- **layout** — a workspace shape. Runtime-shape families are descriptive categories, not
  a `Class` contract.

## Verbs and artifacts

- **scaffold** / **scaffolding** — create the source files a project starts from.
- **render** — run the infra repo's `apps/*.lua` and collect the result into a manifest
  tree ([lua-render.md](lua-render.md)).
- **rock** — a LuaRocks package, fetched from ghcr and pinned under `[rocks]` in
  `platform.toml`. **`infra-lua`** is the rock holding PRODIGY9's Kubernetes library and
  templates.
- **template** — a Lua app file shipped in a rock's `templates/` directory. `init` copies
  it into `apps/`, and the repo owns the copy from then on.
- **export** — write a built image tarball.
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
- **vars** — the `[vars]` table of `platform.toml`. Lua apps read it as `platform.vars`.
- **defaults** — Go defaults are parser-owned config values. A template's defaults are
  the values it declares for its vars, which `init` writes into `[vars]`.
- **`--force`** — override a safety refusal: release accepts a dirty worktree; init
  replaces existing files. **`ALWAYS_YES=1`** answers yes/no confirmation prompts. Neither
  substitutes for the other.
