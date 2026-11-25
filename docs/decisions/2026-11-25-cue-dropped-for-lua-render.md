# CUE is dropped; `render` runs Lua, with `infra-lua` fetched as a rock

- **Date:** 2026-11-25
- **PR:** manual
- **Status:** accepted
- **Supersedes:**
  - [2026-06-16 — Renderer: cue export, not timoni](2026-06-16-renderer-cue-export-not-timoni.md),
    including its rejection of embedded Lua by name.
  - [2026-06-23 — Render via the linked CUE engine](2026-06-23-render-via-linked-cue-engine.md).
  - [2026-07-12 — CUE module path is a scaffold input](2026-07-12-cue-module-path-is-a-scaffold-input.md):
    with no `cue.mod` there is no module path to ask for.
  - The CUE half of
    [2026-06-18 — Render routes .cue and .platform by extension](2026-06-18-render-routes-cue-and-platform-by-extension.md).
    Its other rulings stand: one render, model I (nothing rendered is committed) and the
    file-name-to-documents output contract. Its `.platform` half is superseded by
    [DSL removed for Lua templates](2026-11-25-dsl-removed-for-lua-templates.md).

Current contract: [Lua render](../spec/lua-render.md).

## Decision

`render` evaluates an infra repo's `apps/*.lua` on `gopher-lua`, a pure-Go VM implementing
Lua 5.1, in a sandbox with no file, process, clock or random access. The Kubernetes
definitions move out of CUE (`prodigy9.co/defs`) into a Lua library, `infra-lua`, which
`platform` does not build in: a repo pins its version under `[rocks]` in `platform.toml`,
and `platform` fetches that LuaRocks rock from ghcr with a small built-in OCI fetcher and
caches it at `$XDG_CACHE_HOME/platform/rocks/<name>/<version>/`. A cached version is not
fetched again. The linked CUE engine, `cuemod/`, the CUE dependency fetch and the CUE
scaffold files go.

chakrit ruled it: "Lock in Lua." On the VM: "nah, fine, stick with gopher-lua and lua
5.1." On the cache location: "use xdg folders".

## Rationale

**CUE can no longer run the definitions.** `prodigy9.co/defs` is pinned to CUE v0.15.4.
On CUE v0.18.0, any mixin composed onto `packs.#Gateway` fails in the evaluator
(`#hosts: undefined field: kind`); a bare `parts.#Mixin` with no additions fails the same
way, and no rewrite inside the framework's design was found
(`infra-defs/docs/scratch/2026-11-25-cue-018-assessment.md`). The failure is the evaluator
meeting three things the framework requires at once: dispatch on shape by disjunction in
`#Mixin`, comprehension emit in packs, and patches applied to packs. Staying on v0.15.4
keeps a dependency frozen with no upgrade path. The linked-engine decision stands on CUE
being "our core for a lot of things"; that premise is what failed.

**Why Lua, against the 2026-06-16 rejection.** That record rejected embedded Lua because
"a general-purpose script can't be bounded by reading it". The argument was about patching
foreign manifests with an open language instead of a closed verb set. It is given up
knowingly: the sandbox bounds what an app can reach, if not what it can compute, and the
study of replacements (`infra-defs/docs/scratch/2026-11-25-no-cue-study.md`) compared
Lua, Python, Rust and OCaml, and Lua was the one that runs inside `platform`, keeps
render pure by an allowlist, and needs no toolchain beyond the `platform` binary. Starlark
was noted there but not spiked. A Lua spike rendered a demo repo identically on
Lua 5.4 and `gopher-lua`, rejected eight invalid inputs with file and line, and ran the
mixin-on-Gateway case that fails on CUE v0.16 and later.

**Why Lua 5.1 on `gopher-lua`.** Lua 5.4 and 5.5 VMs for Go were surveyed and set aside.
`gopher-lua` is pure Go, mature, and adds one small dependency; the library is written for
5.1 and its tests run on the same VM.

**Why a fetched rock, not a built-in library.** Building `infra-lua` into `platform` would
version the definitions with the tool and force a `platform` release for every definition
change. A rock pinned per repo keeps the two on their own release lines, as
`prodigy9.co/defs` was on its own module version. ghcr already holds every image platform
publishes and accepts the same `REGISTRY_*` credentials, so the fetch needs no new
registry or credential; the anonymous pull was confirmed by spike.

**Why the XDG cache.** A rock version is immutable once published, so it is fetched once
per machine and kept where per-user caches belong, outside every repo.

## Consequences

- **Evaluation is eager.** A consumer cannot set an input of a definition inside a
  finished pack, as `#components: deploy: #init_command` does in CUE. Packs take such
  inputs directly.
- **Output key order is fixed by rule**, since Lua tables keep no order: `apiVersion`,
  `kind` and `metadata` first, the rest sorted. YAML is written by Go with
  `gopkg.in/yaml.v3`.
- The `@tag` mechanics of
  [render-is-pure-function-of-committed-git](2026-06-26-render-is-pure-function-of-committed-git.md)
  are historical; its ruling that render is a pure function of the committed repo stands,
  now with the pinned rock version as one of the committed inputs.
- Infra repos on defs keep building from the modules already published; moving them to
  Lua is each repo's own work, with a guide.
