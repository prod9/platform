# Lua render

Status: **intended; not implemented.** `render` today evaluates CUE through the linked
CUE engine and runs `.platform` directives (`framework/gitops/render.go`). This spec is
the design that replaces both, for the v1.0 line. The rulings behind it:
[CUE dropped for Lua render](../decisions/2026-11-25-cue-dropped-for-lua-render.md),
[DSL removed for Lua templates](../decisions/2026-11-25-dsl-removed-for-lua-templates.md)
and [schema validation in render](../decisions/2026-11-25-schema-validation-in-render.md).

`render` turns an infra repo's `apps/*.lua` into one manifest tree. It is a pure function
of the committed repo, its `[vars]` table and the pinned rocks; one commit renders the
same tree. The Kubernetes definitions it builds on (kinds, mixins, packs) are not part of
`platform`. They are the `infra-lua` library, shipped as a LuaRocks rock and fetched at
render time.

## Pipeline

1. Read `[rocks]` and `[vars]` from `platform.toml`.
2. Make sure every pinned rock is in the cache, fetching the ones that are not.
3. Run each `apps/<component>.lua` in a fresh sandbox and collect what it returns.
4. Convert every returned document to YAML.
5. Validate every document against the schema for its `apiVersion` and `kind`.
6. Return the tree. Nothing is written before step 6, so a failure at any step leaves no
   output behind.

The `Tree` type (`framework/gitops/tree.go`) and the `platform/infra` framework's
one-layer image build are unchanged; only the steps that produce the tree change.

## The VM and its sandbox

Apps run on [`gopher-lua`](https://github.com/yuin/gopher-lua), a pure-Go VM that
implements Lua 5.1. `infra-lua` is written for Lua 5.1 and its tests run on the same VM.

Each app file gets its own VM state, so nothing one app sets is visible to another. The
state has no access to files, processes, the clock or randomness:

- Only the `base`, `table`, `string` and `math` libraries are opened, with
  `math.random` and `math.randomseed` removed. `io`, `os`, `debug`, `coroutine`,
  `channel` and `package` are not.
- Of the base globals, only these stay: `assert`, `error`, `getmetatable`, `ipairs`,
  `next`, `pairs`, `pcall`, `select`, `setmetatable`, `tonumber`, `tostring`, `type`,
  plus the `string`, `table` and `math` tables. Every other global, including `load`,
  `loadstring`, `loadfile`, `dofile` and `print`, is removed.
- `require` is replaced by platform's own (below) and is the only way to load code.
- `platform` is a global table holding what platform provides: `platform.vars` and the
  upstream download function.

An error raised while an app runs fails the render and names the app file and line.

## `require`

`require "name"` accepts one or more lowercase identifiers joined by dots
(`[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*`, the LuaRocks module-name form, such as
`infra.tree`) and resolves it in this order:

1. A module the rockspec of a pinned rock lists under `build.modules`, loaded from that
   rock's cached directory.
2. `<repo>/<path>.lua`, a module the infra repo owns, where each dot in the name is a
   directory separator, such as its `defaults.lua` of shared definitions.

A name found in neither fails with both places named. A module runs once per VM state;
later `require` calls in the same app return the same value.

## Rocks

A rock is pinned by name and version in a `[rocks]` table in `platform.toml`:

```toml
[rocks]
infra-lua = "v0.1.0"
```

`[rocks]` is a new top-level table in `conf/`'s model. Each entry is one rock, and its
version is the image tag it is fetched by.

**Fetching.** A rock is published as a one-layer OCI image at `ghcr.io/prod9/<name>`, the
image base `conf.InferImageBase` derives for `github.com/prod9/<name>`. `platform` pulls
it with a small built-in OCI fetcher on the Go standard library: token, manifest, the
single layer, a digest check, then a tar+gzip unpack. It sends the `REGISTRY_USERNAME` and
`REGISTRY_PASSWORD` credentials platform already reads for publishing
(`engine/engine.go`) to the registry's token endpoint, and pulls anonymously when they
are unset.

**Cache.** An unpacked rock lives at
`$XDG_CACHE_HOME/platform/rocks/<name>/<version>/` (`~/.cache` when `XDG_CACHE_HOME` is
unset). A version already in the cache is used as it is and never fetched again; a rock
version is immutable once published. The fetch is the `platform/infra` framework's
dependency step, so it is timed apart from evaluation.

**Contents.** The unpacked directory holds the rockspec at its root, the modules it lists
under `build.modules`, and the directory it lists under `copy_directories`, which for
`infra-lua` is `templates/` ([scaffolding](scaffolding.md)).

## `platform.vars`

`platform.vars` carries the `[vars]` table from `platform.toml`. Values keep their TOML
type: a string becomes a Lua string, an integer or float a number, a boolean a boolean.

An app or template declares the vars it uses, with their defaults, by calling it:

```lua
local vars = platform.vars {CERT_MANAGER_VERSION = "v1.20.2"}
```

The call returns a table of exactly the declared names, each holding the `[vars]` value
when one is set and the default otherwise. The declaration is what `init` reads to seed
`[vars]` ([scaffolding](scaffolding.md)): running a template in declare-only mode stops it
at its first `platform.vars` call and reports the names and defaults.

**Open: the case of names.** Today render lowercases every `[vars]` key
(`conf.NormalizeVars`, `conf/conf.go`) because CUE `@tag` holes and directive
interpolation consume the lowercase form, while `platform.toml` and the current defaults
(`infraVars` in `framework/platform_infra.go`) write them in SCREAMING_SNAKE. Whether
`platform.vars` keeps names as written in `platform.toml` or keeps lowercasing them is not
decided.

## Upstream manifests

Third-party installs (cert-manager, Flux, NGINX Gateway Fabric, the Gateway API
definitions) are downloaded at render time by a function platform provides on the
`platform` table. The documents arrive as Lua tables and are edited with ordinary Lua and
`infra-lua`'s helpers, then returned like any other document. The function's name, its
handling of archives (today's DSL `extract`), and whether downloads are cached are not yet
specified.

## Output contract

The contract is the one render has today: component, then file name, then a list of
documents. Each `apps/<component>.lua` is one component, named by its file stem, and
returns a table of file name to documents:

```lua
return {
  ["gateway.yaml"] = gateway,          -- a pack: every resource in it
  ["issuer.yaml"] = issuer,            -- one resource
}
```

A value is one resource, a pack (a list of resources), or a plain list of resources; each
renders as a multi-document file at `<component>/<file name>`. An app that returns
anything else fails the render. The tree is packed into the infra image as exactly one
layer, as today.

## YAML

Go writes the YAML with `gopkg.in/yaml.v3`, already a dependency (`go.mod`); no Lua-side
emitter is used. The conversion from Lua values:

- A table with string keys is a mapping. A table whose keys are exactly `1..n` is a
  sequence. An empty table is a mapping unless `infra-lua` marks it as a list. A table
  mixing both kinds of key fails.
- A number with an integral value is written as an integer, so `replicas = 2` stays `2`.
- Strings and booleans are written as themselves. Functions and other values fail.
- Metatables are ignored: helpers and metadata live there, and none of it is output.

Lua tables keep no key order, so the order is set by rule: in each document `apiVersion`,
`kind` and `metadata` come first, and every other mapping key, at every depth, is sorted.
Sequences keep their order.

## Validation

Before render returns, each document is checked against the schema for its own
`apiVersion` and `kind`:

- **Built-in kinds:** schemas come from Kubernetes' published OpenAPI v3 files.
- **Custom resources:** the schema is the `openAPIV3Schema` embedded in the
  `CustomResourceDefinition` for that group, version and kind.
- Schemas are fetched once into the XDG cache (`$XDG_CACHE_HOME/platform/`) and read from
  there afterwards.
- The check uses [`google/jsonschema-go`](https://github.com/google/jsonschema-go), a
  generic JSON Schema library. `platform` links no Kubernetes library and has no setting
  for a Kubernetes version.

A document that fails its schema fails the render, naming the component, the file, the
document's kind and name, and the failing field. Since validation runs before the tree is
returned, nothing is written and no image is built.

**Open, pending the validation spike:**

- What render does with a document whose kind has no schema available, which happens to a
  tenant repo that uses Gateway API kinds but does not render their CRDs.
- Where the Kubernetes OpenAPI files are fetched from, which Kubernetes release they
  describe without a version setting, and how the cache is laid out.
- How much translation the OpenAPI 3.0 dialect needs before a draft 2020-12 validator
  accepts it (`$ref` paths, `nullable`, `x-kubernetes-int-or-string`,
  `x-kubernetes-preserve-unknown-fields`).

## Templates

`infra-lua` ships copy-pastable Lua files in the rock's `templates/` directory: the
cluster baseline (cert-manager, Flux, the NGINX Gateway with the Gateway API definitions,
the gateway, the ACME cluster issuer and the Flux self-sync) and the shared definitions
module. They are files, not functions the library exposes. `init` copies them into the
repo, and from then on the repo owns them: render runs the repo's copies, not the rock's,
and a newer rock changes nothing in a repo until its files are edited or re-copied.
Each template declares its vars and defaults at the top with `platform.vars`.

## Evaluation is eager

A Lua value is finished when it is built. A pack is a list of resources the moment its
function returns, so a consumer cannot reach into a finished pack and set an input of a
definition inside it, as CUE allows with `#components: deploy: #init_command: [...]`. A
pack that wants such an input takes it directly as one of its own inputs; otherwise the
consumer edits the finished resource in the pack.
