# The `.platform` DSL is removed; the baseline is Lua templates shipped in the rock

- **Date:** 2026-11-25
- **PR:** manual
- **Status:** accepted
- **Supersedes:**
  - [2026-06-20 — DSL: focus scope, strict values](2026-06-20-dsl-focus-scope-strict-values.md).
  - The `.platform` half of
    [2026-06-18 — Render routes .cue and .platform by extension](2026-06-18-render-routes-cue-and-platform-by-extension.md).
    Its CUE half is superseded by
    [CUE dropped for Lua render](2026-11-25-cue-dropped-for-lua-render.md); its rulings on
    one render, model I and the output contract stand.

Current contracts: [Lua render](../spec/lua-render.md) and
[scaffolding](../spec/scaffolding.md).

## Decision

The manifest-patch DSL (`dsl/`, the `.platform` files and their render route) is removed.
The cluster baseline it expressed becomes pre-built templates: copy-pastable Lua files in
the `templates/` directory of the `infra-lua` rock. `init` copies them into the repo,
which then owns them; render runs the repo's copies like any other app. Each template
declares its vars and defaults at the top with
`local vars = platform.vars {NAME = "default"}`.

The templates are files, not functions the library exposes. A third-party install is
downloaded through a function `platform` provides and edited with ordinary Lua.

chakrit ruled it: "DSL is removed, replace with some pre-built templates", and "by
template i meant lua files". The templates ship with the rock rather than being baked into
`infra-lua` functions.

## Rationale

The DSL existed because CUE could not append to a concrete list, and because a closed
verb set could be bounded by reading it while a general-purpose script could not
([2026-06-16](2026-06-16-renderer-cue-export-not-timoni.md)). With render on Lua, the
first reason is gone and the second was given up knowingly. A second, smaller language
beside Lua would then only add a grammar, a parser and a per-repo law gating every change
to it, for edits Lua already expresses: find a document by kind and name, set a field,
append to a list.

**Why files, not library functions.** A baseline component as a library function would
put the operator's cluster setup behind the rock's version: changing one flag would mean a
new rock release or an option on the function. A copied file is the write-once,
operator-owns shape the baseline already had
([2026-06-17](2026-06-17-opinionated-appliance-embedded-init.md)). Provider wiring stays
the repo's own edit to its copy
([2026-07-16](2026-07-16-baseline-is-provider-neutral.md)).

**Why in the rock, not embedded in `platform`.** The templates are written against
`infra-lua` and change with it; shipping them in the same rock keeps a template and the
library it calls on one version, and leaves `platform` free of baseline content.

## Consequences

- The baseline installs whole and unconditionally, as before
  ([2026-07-11](2026-07-11-baseline-dissolves-into-infra-framework.md)); only its source
  moves from `framework/skel/` to the rock.
- `[vars]` stays a generic open table
  ([2026-06-17](2026-06-17-generic-ops-vars-single-config.md)); templates read it through
  `platform.vars` instead of `\(var)` interpolation.
- The per-repo law hard-gating DSL changes has no subject left.
- The template that deployed the platform server is not carried over
  ([server removed](2026-11-25-server-removed-platform-is-a-cli.md)).
