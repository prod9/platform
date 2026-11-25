# Lua render

Status: **implemented, except the upstream download function and the declare-only run
`init` uses (intended).** `render` runs Lua only
(`framework/gitops/`); CUE and the `.platform` directives are gone. The rulings behind
it:
[CUE dropped for Lua render](../decisions/2026-11-25-cue-dropped-for-lua-render.md),
[DSL removed for Lua templates](../decisions/2026-11-25-dsl-removed-for-lua-templates.md),
[validation left to Flux](../decisions/2026-11-25-validation-left-to-flux.md) and
[rocks dropped for subtree vendoring](../decisions/2026-11-25-rocks-dropped-for-subtree-vendoring.md).

`render` turns an infra repo's `apps/*.lua` into one manifest tree. It is a pure function
of the committed repo and its `[vars]` table; one commit renders the same tree. The
Kubernetes definitions it builds on (kinds, mixins, packs) are not part of `platform`.
They are the `infra-lua` library, which the infra repo vendors as a git subtree at
`vendor/infra-lua` ("Vendored libraries", below), so they are committed files like the
repo's own: render fetches no library, reads no registry and needs no credential for one.

## Pipeline

1. Read `[vars]` from `platform.toml`.
2. Index the modules of every vendored library under `vendor/`, failing on a module name
   two libraries both provide.
3. Run each `apps/<component>.lua` in a fresh sandbox and collect what it returns.
4. Convert every returned document to YAML.
5. Return the tree. Nothing is written before step 5, so a failure at any step leaves no
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
  `next`, `pairs`, `pcall`, `rawget`, `select`, `setmetatable`, `tonumber`, `tostring`,
  `type`,
  plus the `string`, `table` and `math` tables. Every other global, including `load`,
  `loadstring`, `loadfile`, `dofile` and `print`, is removed.
- `require` is replaced by platform's own (below) and is the only way to load code.
- `platform` is a global table holding what platform provides: `platform.vars` and the
  upstream download function.

An error raised while an app runs fails the render and names the app file and line.

## `require`

`require "name"` accepts one or more lowercase identifiers joined by dots
(`[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*`, Lua's dotted module-name form, such as
`defs.tree`). Each dot in the name is a directory separator, giving a path such as
`defs/tree`, and the name resolves in this order:

1. `<repo>/vendor/<lib>/src/<path>.lua` in a vendored library, where `<lib>` is any
   directory directly under `vendor/`: `require "defs.tree"` loads
   `vendor/infra-lua/src/defs/tree.lua`, and `require "defs"` loads
   `vendor/infra-lua/src/defs.lua`.
2. `<repo>/<path>.lua`, a module the infra repo owns, such as its `defaults.lua` of
   shared definitions.

Two vendored libraries that provide the same module name fail the render before any app
runs; directory order never picks one. A library directory with no `src/` provides no
module, and a file under `src/` whose path no dotted name maps to (such as `a.b.lua`) is
not a module. A name found nowhere fails, naming every path searched. A module runs once per VM state; later `require` calls in the same app return
the same value.

## Vendored libraries

A vendored library is a git repository committed whole under `vendor/<lib>/` as a git
subtree. Render reads one fixed root in it, `src/`, and nothing else; everything else the
library carries (its tests, docs, templates) is inert to render. There is no
configuration for it: a directory under `vendor/` is a library, and its absence means no
library.

`infra-lua` is vendored from `https://github.com/prod9/infra-lua` at one of its release
tags, with `--squash`:

```sh
git subtree add --prefix=vendor/infra-lua https://github.com/prod9/infra-lua <tag> --squash
```

`platform init` runs that command on a repo that has no `vendor/infra-lua`
([scaffolding](scaffolding.md)). infra-lua releases are plain git tags; nothing is
published to a registry. Upgrading is the operator's own
`git subtree pull --prefix=vendor/infra-lua https://github.com/prod9/infra-lua <tag> --squash`,
committed like any other change; platform never bumps a vendored library.

## `platform.vars`

`platform.vars` carries the `[vars]` table from `platform.toml`. Values keep their TOML
type: a string becomes a Lua string, an integer or float a number, a boolean a boolean.
Any other TOML value (an array, a table, a date) fails the render before any app runs.

An app or template declares the vars it uses, with their defaults, by calling it:

```lua
local vars = platform.vars {CERT_MANAGER_VERSION = "v1.20.2"}
```

The call returns a table of exactly the declared names, each holding the `[vars]` value
when one is set and the default otherwise. The declaration is what `init` reads to seed
`[vars]` ([scaffolding](scaffolding.md)): running a template in declare-only mode stops it
at its first `platform.vars` call and reports the names and defaults.

**Names are kept as written in `platform.toml`.** `CERT_MANAGER_VERSION` in `[vars]` is
`CERT_MANAGER_VERSION` in Lua, and a declaration matches a `[vars]` key only when the two
are spelled the same.

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
  sequence. An empty table is a mapping unless it carries `infra-lua`'s list metatable
  (the one `defs.tree.list` sets); then it is an empty sequence. A table mixing both
  kinds of key fails.
- Render takes the list metatable from the app's own VM state, since each state loads
  its own copy of the module: after the app returns, it calls
  `getmetatable(require("defs.tree").list({}))` there and recognizes the mark by
  identity, whichever root `defs.tree` resolves from. When `defs.tree` does not resolve,
  there is no mark and every empty table is a mapping.
- A number with an integral value is written as an integer, so `replicas = 2` stays `2`.
- Strings and booleans are written as themselves. Functions and other values fail.
- Apart from the list mark, metatables are ignored: helpers and metadata live there, and
  none of it is output.

Lua tables keep no key order, so the order is set by rule: in each document `apiVersion`,
`kind` and `metadata` come first, and every other mapping key, at every depth, is sorted.
Sequences keep their order.

## No schema validation

Render does not check documents against Kubernetes schemas. What it checks is what
`infra-lua` checks: input names, required inputs and Lua types, raised as errors that fail
the render. A document that is wrong for the cluster, such as a misspelled field in a raw
resource or an edited upstream manifest, is rejected by the cluster's API server when
Flux applies it: the `Kustomization` reports `Ready=False` with the error and retries
until a fixed tree is published. The reasoning is in
[validation left to Flux](../decisions/2026-11-25-validation-left-to-flux.md).

## Templates

`infra-lua` ships copy-pastable Lua files in its `templates/` directory, vendored with it
at `vendor/infra-lua/templates/`: the
cluster baseline (cert-manager, Flux, the NGINX Gateway with the Gateway API definitions,
the gateway, the ACME cluster issuer and the Flux self-sync) and the shared definitions
module. They are files, not functions the library exposes. `init` copies them into the
repo, and from then on the repo owns them: render runs the repo's copies, never the
vendored originals, and a newer vendored `infra-lua` changes nothing in the copies until
they are edited or re-copied.
Each template declares its vars and defaults at the top with `platform.vars`.

## Evaluation is eager

A Lua value is finished when it is built. A pack is a list of resources the moment its
function returns, so a consumer cannot reach into a finished pack and set an input of a
definition inside it, as CUE allows with `#components: deploy: #init_command: [...]`. A
pack that wants such an input takes it directly as one of its own inputs; otherwise the
consumer edits the finished resource in the pack.
