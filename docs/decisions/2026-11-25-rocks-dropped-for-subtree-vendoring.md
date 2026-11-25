# Rocks are dropped; infra repos vendor `infra-lua` as a git subtree

- **Date:** 2026-11-25
- **PR:** manual
- **Status:** accepted
- **Supersedes:**
  - The rock half of
    [2026-11-25 — CUE is dropped; render runs Lua with infra-lua fetched as a rock](2026-11-25-cue-dropped-for-lua-render.md):
    the `[rocks]` pin, the ghcr fetch with `REGISTRY_*` credentials, the XDG rock cache,
    and its "Why a fetched rock" and "Why the XDG cache" reasoning. Its Lua render
    rulings (CUE dropped, `gopher-lua` and Lua 5.1, eager evaluation, fixed key order)
    stand.
  - The "shipped in the rock" placement of
    [2026-11-25 — The `.platform` DSL is removed; the baseline is Lua templates](2026-11-25-dsl-removed-for-lua-templates.md).
    The templates still ship with `infra-lua`, now in its vendored `templates/`; the
    ruling that they are copied files the repo owns stands.

Current contracts: [Lua render](../spec/lua-render.md) ("`require`", "Vendored
libraries") and [scaffolding](../spec/scaffolding.md) ("The `Infra` framework scaffolds
from vendored `infra-lua`").

## Decision

`platform` has no rocks. There is no `[rocks]` table in `platform.toml`, no `rocks`
package or OCI fetcher, no rock cache, no registry pull or credentials on any render
path, no LuaRock build framework and no rock publish. `infra-lua` releases are plain git
tags.

An infra repo vendors the whole `infra-lua` repository as a git subtree at
`vendor/infra-lua`, from `https://github.com/prod9/infra-lua` at a tag, with `--squash`.
`render`'s `require` searches `vendor/<lib>/src/<path>.lua` for each directory directly
under `vendor/`, then the repo's own `<path>.lua`; one module name in two vendored
libraries fails the render. `infra-lua`'s modules live under `src/defs` (`defs.tree`,
kinds flat as `defs.<kind>`), with the by-kind aggregator at `src/defs.lua`
(`require "defs"`).

`platform init` on an infra repo without `vendor/infra-lua` resolves the newest tag with
`git ls-remote`, asks a yes/no confirmation, and runs `git subtree add`; that is the only
commit `init` makes. An existing `vendor/infra-lua` is never bumped; upgrading is the
operator's own `git subtree pull`.

chakrit ruled it: "i'm thinking perhaps we go back to the old subtree approach and just
vendor the whole thing as subtree, then completely remove rocks from platform", then "i'd
rather infra-lua just have regular git tags. but yeah let's do this." On the module
folder: "i'd rather that folder be named `defs` so src/defs so it matches cue prior" and
"fine to place defs.lua". On init: "step 2) should probably be a prompt from platform to
invoke git command to do it. (i.e. bake in the git subtree thing)".

## Rationale

**Render reads only committed files.**
[Render is a pure function of committed git](2026-06-26-render-is-pure-function-of-committed-git.md)
holds that the committed repo determines the rendered output. A pinned rock met that only
by reference: the pin was committed, but the code it named lived in a registry and a
per-machine cache, so rendering a commit depended on the registry still serving that
version and on the cache holding what the registry served. With the library vendored, the
code render runs is in the commit itself, reviewable in the same diff as the apps that
use it.

**No registry and no token on any build path.** Fetching a rock from ghcr meant render
and `init` carried registry credentials, and ghcr accepts only a classic PAT outside
Actions ([ghcr auth](../vendor/ghcr-auth.md)). A subtree is fetched by the operator's own
git, once, when it is added or pulled; after that, rendering touches no registry for the
library.

**`platform` gets smaller.** The rock fetcher, its cache, the `[rocks]` table and its
merge, the LuaRock framework and the rock publish all go. Versioning the library apart
from the tool, the reason the rock existed, is kept: the vendored copy moves only when the
operator pulls a new tag, and an `infra-lua` change needs no `platform` release.

**Why not the obvious package-manager route.** A pinned package fetched at build time is
the mainstream answer, and it is what the rock was. It costs a registry, a credential, a
fetcher and a cache in exchange for a smaller repo; for one internal library of Lua
source, the vendored copy is the cheaper side of that trade.
