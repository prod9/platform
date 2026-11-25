# Migration — legacy platform → pull-based v2

Living checklist of breaking changes as the pull-based rework lands, and how a consuming repo
moves across each. Newest first.

## v1.0: Lua render, no server

platform v1.0 is a CLI only, and an infra repo renders Lua instead of CUE. App repos need
nothing beyond bumping the launcher pin. The design is in
[`../spec/lua-render.md`](../spec/lua-render.md) and
[`../spec/scaffolding.md`](../spec/scaffolding.md).

What is gone:
- `platform srv` and `platform worker`, the web UI and webhook: there is no server. Builds
  and publishes run where the CLI runs.
- CUE and `prodigy9.co/defs` (archived): `render` runs `apps/*.lua` only, and `cue.mod/`
  is no longer read.
- The `.platform` manifest-patch DSL: upstream installs are Lua files that call
  `platform.download(url)` and edit the documents with plain Lua.
- Schema validation at render: a document that is wrong for the cluster is rejected when
  Flux applies it.

Migrate an infra repo:
1. Commit everything, then vendor the library:
   `git subtree add --prefix=vendor/infra-lua https://github.com/prod9/infra-lua <tag> --squash`.
   On a repo with no baseline yet, `platform init` does this and copies the baseline
   templates for you.
2. Rewrite each `apps/<component>.cue` as `apps/<component>.lua`, returning a table of
   file name to documents. The kinds keep their CUE names on `require "defs"`
   (`defs.Deployment{...}`), and the mixins and packs are `require "defs.mixins"` and
   `require "defs.packs"`; shared definitions move
   from `defaults/` to a repo-root `defaults.lua`, loaded with `require "defaults"`. The
   library's surface is in infra-lua's `docs/spec/`.
3. Replace each `.platform` file with the matching infra-lua template from
   `vendor/infra-lua/templates/`, then carry over your own edits in Lua.
4. `[vars]` names now reach Lua exactly as written: `CERT_MANAGER_VERSION` is
   `vars.CERT_MANAGER_VERSION` after `local vars = platform.vars {CERT_MANAGER_VERSION = "v1.20.2"}`,
   not the lowercased `cert_manager_version` CUE and the DSL saw. Only strings, numbers
   and booleans are allowed.
5. Delete `cue.mod/` and the `.cue` and `.platform` files, then `platform render` and
   compare the tree with the last CUE render before publishing.

## Environments are no longer platform-managed

The `environments` list (`platform.toml`) and the `deploy` verb are **removed**. Platform no
longer owns any notion of an environment.

Multi-environment is now expressed **manually in the infra repo's CUE** — value templating +
k8s namespacing: a shared template (e.g. `apps/template/`) instantiated once per environment
(`apps/dev`, `apps/prod`, …), each landing in its own namespace. No platform enum, no
`platform deploy`.

Migrate:
- Drop `environments = [...]` from `platform.toml`.
- Model each environment as a CUE instance of a shared template, separated by namespace.
- Deploy = commit the infra repo, then `publish` (Flux pulls the image) **or** `render` +
  `kubectl apply`. The gate is the infra repo's GitHub push permissions — whoever can push
  can deploy.
