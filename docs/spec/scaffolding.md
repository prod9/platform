# Scaffolding

Status: **design-of-record; the infra baseline is intended.** Covers `platform init` — how
a fresh repo is seeded. The infra baseline described under "The `Infra` framework
scaffolds from vendored `infra-lua`" is the v1.0 design and replaces the embedded CUE and
`.platform` baseline, which goes with CUE and the DSL. Until it is built, `Infra.Scaffold`
contributes only its module and its `strategy` seed: no vendoring, no files, no `[vars]`.
The rest of this file is implemented. New code conforms;
existing code migrates toward it.

Scaffolding has exactly three concerns, each with one home:

- **`framework/scaffold/`** — the one files/templating mechanism: resolve templates
  with data. Generic; no discover, no orchestration, no per-type data, no writes — the
  driver writes finished bytes.
- **The `Infra` framework** — owns the cluster baseline it scaffolds. The baseline's files
  are the templates of `infra-lua`, vendored at `vendor/infra-lua`
  ([Lua render](lua-render.md)); the framework names `infra-lua` as the library it needs
  (`Vendors`), routes the templates and collects their declared vars.
  `framework/skel` keeps only the universal launcher.
- **`scaffolding/`** — the initialization domain: target validation, framework discovery,
  operator input data, plan construction, and file application. The thin Cobra adapter is
  `cmd/init_cmd.go` and owns prompt/session wiring; the domain package owns no command
  registration.

A `Framework` is the sole owner of a project type, so what a repo scaffolds is
`fw.Scaffold`'s output — nothing branches on app-vs-infra. An **app repo** gets a
`platform.toml` + executable `platform` launcher; an **infra repo** gets that base plus
the vendored `infra-lua`, the cluster baseline (`apps/*.lua` and the shared
`defaults.lua`) and its declared `[vars]`. The difference is only how much
`Infra.Scaffold` contributes — pure polymorphism, **no `IsInfra` predicate**.

The launcher is the driver's own contribution, so the driver resolves its one hole the
same way frameworks resolve theirs: `PLATFORM_VERSION` is stamped with
`framework.PlatformVersion()` — the valid SemVer value in the running binary's build
metadata, verbatim. This implemented contract replaces predecessor-release recovery:
validate SemVer only; do not canonicalize, strip suffixes, recover predecessors, or check
publication. Missing or invalid SemVer is a hard init error.

Launcher generation targets publicly released builds, versions, and tags. For an
in-development build, the pin may name a version that does not exist publicly; no
fallback or launcher development mode is provided. Development testing uses an installed
binary directly, for example `go install platform.prodigy9.co@<commit>`, rather than a
pinned launcher.

### Installing

The launcher installs platform with `go install platform.prodigy9.co@<version>`.
`platform.prodigy9.co` is a vanity module path: Go asks
`https://platform.prodigy9.co/<path>?go-get=1` for a page whose
`<meta name="go-import" content="platform.prodigy9.co git https://github.com/prod9/platform">`
names the repository. platform does not serve that page and publishes no image of
itself. A Cloudflare Worker on the `platform.prodigy9.co` hostname, managed by OpenTofu
in `infra-tf`, answers every path with it
([go-import page served by Cloudflare](../decisions/2026-11-25-go-import-page-served-by-cloudflare.md)).
This is **intended**; until the Worker is live, the v0.x platform server still running on
`prod9-main` answers the lookup.

For the pipeline this feeds, see [architecture](architecture.md); the `Framework` contract
and scaffold-time stack discovery live in [frameworks](frameworks.md) (the build path reads
`[modules]`, never re-discovers).

## `framework/scaffold/` — the one mechanism

`framework/scaffold/` resolves a framework's contribution. It defines the two
shapes a framework returns and nothing stack-specific:

- **`scaffold.Spec`** — a framework's full contribution to a fresh repo: the
  `platform.toml` module it adds, the default `[vars]` it seeds, the files it ships
  (`[]scaffold.File`, already **resolved** — the framework fills its own holes), and the
  default `strategy` value it seeds.
- **`scaffold.File`** — one file beyond the universal `platform.toml` + launcher: `Path`
  (repo-relative, routing already applied), `Content`, `Mode`. A `.tmpl` suffix marks
  `Content` as a `text/template` the mechanism resolves (and strips) with the
  scaffold data.
- **`scaffold.Vendor`** — a git-subtree library a framework needs in the repo before it
  can scaffold: `Prefix` (repo-relative, such as `vendor/infra-lua`) and `URL` (the git
  repository it is vendored from). Pure data; the `scaffolding/` flow does the vendoring.

**Templating rules.** `.tmpl` files resolve through `text/template` with `missingkey=error`;
non-template files pass through **verbatim**. The launcher is the one `.tmpl` file left.
`infra-lua`'s Lua templates are never `.tmpl` files and never meet the template engine:
their values come from `[vars]` at render time, not from holes filled at init. Output
order is deterministic.

The mechanism is generic: it does not discover, does not orchestrate, and holds no per-type
data. Everything type-specific comes in through the `scaffold.Spec` a framework hands it.

## The `Infra` framework scaffolds from vendored `infra-lua`

The cluster baseline is not embedded in `platform`. It ships as the `templates/` directory
of `infra-lua`, which the infra repo vendors whole as a git subtree at `vendor/infra-lua`
([Lua render](lua-render.md), "Vendored libraries"): copy-pastable Lua files a repo copies
and then owns
([DSL removed for Lua templates](../decisions/2026-11-25-dsl-removed-for-lua-templates.md)).
`init` vendors the library when the repo lacks it, then makes the copy.

`Infra.Vendors` returns one `scaffold.Vendor`: prefix `vendor/infra-lua`, URL
`https://github.com/prod9/infra-lua`. By the time `Infra.Scaffold` runs, the
`scaffolding/` flow has vendored it (`scaffolding/`, step 3), so `Infra.Scaffold` only
reads:

1. **Reads the templates** from `vendor/infra-lua/templates/`.
2. **Routes every template.** The shared definitions module goes to the repo root
   (`defaults.lua`, which apps load with `require "defaults"`); every other template goes
   to `apps/`. Contents are copied byte for byte.
3. **Collects the declared vars.** It runs each template in declare-only mode: the
   template stops at its first `platform.vars {…}` call, and the names and defaults it
   declares become the default `[vars]` the framework seeds. Two templates that declare
   one name with different defaults fail init.

`Infra.Scaffold` asks for no scaffold input; `ScaffoldVars` reports none. Every value the
baseline needs is a declared var with a default, so `init` prompts for nothing beyond the
universal maintainer, email and repository.

The baseline installs **whole and unconditionally** — one flat list, no picker, no marker
grammar, no render-time gating
([baseline-dissolves-into-infra-framework](../decisions/2026-07-11-baseline-dissolves-into-infra-framework.md)).
An operator who wants a smaller baseline deletes files after init. There is no checklist
step.

The default working set is the set of templates in the vendored `infra-lua`: cert-manager
(Gateway API and ListenerSet reconciliation enabled — the cross-cloud convention), Flux,
the Flux self-sync (the GitHub→Flux `Receiver` with its own `ListenerSet` and route), the
NGINX Gateway with the Gateway API definitions, the host-agnostic operator `Gateway` app,
and the ACME cluster issuer. Components own their hostnames via `ListenerSet`s; the
gateway app carries none. The baseline deploys no platform server and no Dagger engine
pool ([server removed](../decisions/2026-11-25-server-removed-platform-is-a-cli.md)).

The baseline stays provider-neutral
([baseline-is-provider-neutral](../decisions/2026-07-16-baseline-is-provider-neutral.md)):
cloud wiring is the infra repo's own edit to its copies.

### Multi-tenant clusters: baseline owner vs tenant (convention, not code)

A shared Flux cluster has **one baseline owner** (the repo that scaffolds this full baseline)
and **N tenants** (other repos delivering apps onto the same cluster). The distinction is a
**convention operators follow when committing a tenant repo**, never scaffolding machinery —
`Infra.Scaffold` stays fixed and unconditional:

- **Baseline owner** emits the whole set above — including the `Receiver`, whose target
  namespaces enumerate every reconciled namespace (owner + each tenant). Onboarding a
  tenant appends that tenant's namespace to the owner's list. No per-tenant Receiver,
  webhook, or HMAC secret exists.
- **Tenant** commits **only** its own `OCIRepository` + `Kustomization` in its own namespace
  (the `FluxTenant` pack). It **must not** emit any baseline component (gateway /
  cert-manager / flux-system) — a double-emit gives the shared cluster two gateways /
  cert-managers / flux installs that prune each other. An operator scaffolding a tenant
  prunes the baseline down to those two resources; the pruning is the operator's, not a
  scaffold mode.

### Declared vars: interpolation inputs, not selection

The baseline's default `[vars]` are what its templates declare: version pins
(`CERT_MANAGER_VERSION`, `FLUX_VERSION`, …) and per-deployment values such as the Flux
receiver host. They are inputs to the templates, read through `platform.vars`.
**Selection is not a var** — the full baseline installs unconditionally.

## `scaffolding/` — initialization domain

`cmd/init_cmd.go` (aliases `init` and `scaffold`) drives the flow through `scaffolding/`,
which owns every mutation. It is **discover-then-vendor-then-plan-then-apply**: discovery
validates the target and resolves its framework once; vendoring commits the libraries the
framework needs; planning reads only; the command confirms each mutation before it
happens.

1. `scaffolding.Discover(wd)` validates the target: it exists, is a directory, and is its
   own git repo root
   (`git.IsRoot` — a `.git` **directly** in `wd`, no walk-up, so a standalone repo nested
   inside another checkout counts only with its own `.git`).
2. The same call resolves the owning framework and returns a `Target` that retains it. The
   command prompts for the additional inputs `Target.ScaffoldVars()` reports.
3. **Vendoring.** For each `scaffold.Vendor` the framework's `Vendors` returns whose
   `Prefix` does not exist in the repo (an existing one is skipped and never bumped):
   1. The target checks the precondition `git subtree add` holds the repo to: the current
      branch has at least one commit, and no tracked file has staged or unstaged changes.
      Untracked files do not count. A repo that fails it fails init with an error that
      names the missing commit or the changed files, before anything is asked or written;
      the operator commits and re-runs `init`.
   2. It resolves the newest tag with `git ls-remote --tags <URL>`. Newest is the highest
      SemVer precedence among the tags that are valid SemVer
      (`vMAJOR.MINOR.PATCH[-PRE]`), prereleases included, the ordering
      [releases](releases.md) ("Collection") applies: `v0.2.0` sorts above
      `v0.2.0-alpha.1`, and `v0.3.0-alpha.1` above both. There is no separate stable
      channel. No such tag fails init.
   3. The command asks one yes/no confirmation (fx `prompts.Confirm`, which
      `ALWAYS_YES=1` answers) naming the tag and the command. Declining stops init with
      nothing written.
   4. The target runs `git subtree add --prefix=<Prefix> <URL> <tag> --squash`. `git
      subtree` missing from the operator's git fails init with an error naming
      `git subtree`. Any other failure of the command fails init.

   Each subtree add is a commit `init` makes, and the only kind: git subtree records it as
   a merge commit on the current branch whose second parent is the squashed content of
   the tag. Everything else `init` writes is left uncommitted. Vendoring runs before
   planning because `Scaffold` reads the vendored files. Upgrading a vendored library is
   the operator's own `git subtree pull` ([Lua render](lua-render.md), "Vendored
   libraries").
4. `Target.Plan(info, vars)` calls the retained framework's `Scaffold` once. Its
   `scaffold.Spec` carries the module, vars, resolved files, and seeded `strategy`;
   the target computes the `platform.toml` disposition and returns a read-only plan.
5. The command prints and confirms the plan, then calls `Plan.Apply()` or, for `--force`,
   `Plan.ForceApply()`.

**The git precondition is uniform, not framework-set.** Platform never runs `git init` —
the operator creates the repo (or clones) first, for every framework alike: delivery is
git-based end to end, so a non-repo target is always a mistake.

The plan carries `Files []FileChange` and `Vars []VarChange`. Each `FileChange` records
`FileWrite` vs `FileOverwrite` (decided by an existence stat at plan time) so `Print` can
warn before an overwrite. `Apply` writes, **skipping overwrites**; `ForceApply` replaces in
place. `Overwrites()` counts replacements so `init` prompts only when some exist. `init`
closes by encoding the effective parsed config (`conf.Load`, same view as
`configure`) so the operator sees the resolved result of the freshly written `platform.toml`.

### Non-interactive drive

Drive `init` non-interactively with positional values **and** `ALWAYS_YES=1`. `--force` is
orthogonal:

- Positional arguments supply the value prompts in order: maintainer, email, repository,
  then any framework `ScaffoldVars`.
- `ALWAYS_YES=1` — the fx prompt session auto-answers only yes/no confirmation gates.
- `--force` — selects `ForceApply`: **replace existing files** instead of keeping them.
  Purely about the write disposition, not about suppressing prompts.

### `platform.toml` disposition

- **Absent** → a fresh file is generated from `conf.ModelDefaults`, the operator `Info`
  (maintainer, repository), the framework's `scaffold.Spec` module (its `[modules]` entry and
  the `strategy` value it seeds — `Infra` seeds `strategy = "rolling"` since infra has no
  versions to cut), and the seeded default `[vars]`.
- **Present** → the surgical merge (below) folds the default `[vars]` in **textually**;
  every other table, comment, and byte is preserved.

### Re-init: surgical merge

Planning a re-init computes the surgical merge below. Applying the plan while keeping
existing files leaves `platform.toml` untouched, so the planned additions are written only
when replacement is accepted (`--force` or the overwrite confirmation). Replacement uses
the merged bytes, preserving the operator's existing config.

The merge (owned by `conf/`, alongside `Generate`) folds the defaults into `[vars]`
**line-by-line, not by decode/re-encode** — a round-trip through the TOML encoder would
lose the operator's comments, ordering, and formatting. The merge:

- **Appends** default keys the file lacks, inserted after the last non-blank line of the
  existing table body (or a fresh table appended to EOF when absent).
- **Preserves** keys already present — the operator's value stands untouched.
- Leaves comments, key order, and all other tables byte-for-byte.

Each default var's disposition is recorded as a `VarChange` (`Appended` true = newly
added, false = operator value kept), surfaced in the plan. Values keep their TOML type on the
append (strings quoted/escaped, bools and numbers bare). Keeping existing files also
preserves edits to the copied templates. When replacement is accepted, the templates are
re-copied whole and edits to them are overwritten; durable customization belongs in vars
or in files the operator added.

## Invariants — do NOT re-litigate

- **`apps/` is render-only.** Every `apps/<component>.lua` is one `render` component.
  Shared definitions do not live here — they live in repo-root modules such as
  `defaults.lua`, loaded with `require`.
- **The templates are the repo's files after init.** Render runs the repo's copies, never
  the vendored originals; a newer vendored `infra-lua` changes nothing in the copies until
  they are edited or re-copied.
- **No baseline is embedded in `platform`.** The baseline's source of truth is
  `infra-lua`'s `templates/`; `framework/skel` holds only the launcher.
