# Scaffolding

Status: **design-of-record; the infra baseline is intended.** Covers `platform init` — how
a fresh repo is seeded. The infra baseline described under "The `Infra` framework
scaffolds from the rock" is the v1.0 design and replaces the embedded CUE and `.platform`
baseline, which goes with CUE and the DSL. Until it is built, `Infra.Scaffold` contributes
only its module and its `strategy` seed: no files, no `[vars]`, no `[rocks]` pin. The rest
of this file is implemented. New code conforms;
existing code migrates toward it.

Scaffolding has exactly three concerns, each with one home:

- **`framework/scaffold/`** — the one files/templating mechanism: resolve templates
  with data. Generic; no discover, no orchestration, no per-type data, no writes — the
  driver writes finished bytes.
- **The `Infra` framework** — owns the cluster baseline it scaffolds. The baseline's files
  are the templates of the pinned `infra-lua` rock ([Lua render](lua-render.md)); the
  framework fetches the rock, routes the templates and collects their declared vars.
  `framework/skel` keeps only the universal launcher.
- **`scaffolding/`** — the initialization domain: target validation, framework discovery,
  operator input data, plan construction, and file application. The thin Cobra adapter is
  `cmd/init_cmd.go` and owns prompt/session wiring; the domain package owns no command
  registration.

A `Framework` is the sole owner of a project type, so what a repo scaffolds is
`fw.Scaffold`'s output — nothing branches on app-vs-infra. An **app repo** gets a
`platform.toml` + executable `platform` launcher; an **infra repo** gets that base plus
the cluster baseline (`apps/*.lua` and the shared `defaults.lua`), its declared `[vars]` and
the `[rocks]` pin. The difference is only how much `Infra.Scaffold` contributes — pure
polymorphism, **no `IsInfra` predicate**.

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
  `platform.toml` module it adds, the default `[vars]` it seeds, the rocks it pins, the
  files it ships (`[]scaffold.File`, already **resolved** — the framework fills its own
  holes), and the default `strategy` value it seeds.
- **`scaffold.File`** — one file beyond the universal `platform.toml` + launcher: `Path`
  (repo-relative, routing already applied), `Content`, `Mode`. A `.tmpl` suffix marks
  `Content` as a `text/template` the mechanism resolves (and strips) with the
  scaffold data.

**Templating rules.** `.tmpl` files resolve through `text/template` with `missingkey=error`;
non-template files pass through **verbatim**. The launcher is the one `.tmpl` file left.
The rock's Lua templates are never `.tmpl` files and never meet the template engine: their
values come from `[vars]` at render time, not from holes filled at init. Output order is
deterministic.

The mechanism is generic: it does not discover, does not orchestrate, and holds no per-type
data. Everything type-specific comes in through the `scaffold.Spec` a framework hands it.

## The `Infra` framework scaffolds from the rock

The cluster baseline is no longer embedded in `platform`. It ships as the `templates/`
directory of the `infra-lua` rock: copy-pastable Lua files a repo copies and then owns
([DSL removed for Lua templates](../decisions/2026-11-25-dsl-removed-for-lua-templates.md)).
`init` is the copy.

`Infra.Scaffold` on an infra repo:

1. **Resolves the rock version.** On a repo with no `[rocks]` pin for `infra-lua`, it
   reads the tag list of `ghcr.io/prod9/infra-lua` and takes the newest published version. An
   existing pin is the operator's value and is kept.
2. **Fetches the rock** into the cache with the same fetcher and credentials render uses,
   and reads the files under its `templates/` ([Lua render](lua-render.md), "Rocks").
3. **Routes every template.** The shared definitions module goes to the repo root
   (`defaults.lua`, which apps load with `require "defaults"`); every other template goes
   to `apps/`. Contents are copied byte for byte.
4. **Collects the declared vars.** It runs each template in declare-only mode: the
   template stops at its first `platform.vars {…}` call, and the names and defaults it
   declares become the default `[vars]` the framework seeds. Two templates that declare
   one name with different defaults fail init.
5. **Pins the version** under `[rocks]`.

`Infra.Scaffold` asks for no scaffold input; `ScaffoldVars` reports none. Every value the
baseline needs is a declared var with a default, so `init` prompts for nothing beyond the
universal maintainer, email and repository.

The baseline installs **whole and unconditionally** — one flat list, no picker, no marker
grammar, no render-time gating
([baseline-dissolves-into-infra-framework](../decisions/2026-07-11-baseline-dissolves-into-infra-framework.md)).
An operator who wants a smaller baseline deletes files after init. There is no checklist
step.

The default working set is the set of templates in the rock: cert-manager (Gateway API and
ListenerSet reconciliation enabled — the cross-cloud convention), Flux, the Flux self-sync
(the GitHub→Flux `Receiver` with its own `ListenerSet` and route), the NGINX Gateway with
the Gateway API definitions, the host-agnostic operator `Gateway` app, and the ACME
cluster issuer. Components own their hostnames via `ListenerSet`s; the gateway app carries
none. The baseline deploys no platform server and no Dagger engine pool
([server removed](../decisions/2026-11-25-server-removed-platform-is-a-cli.md)).

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
which owns every mutation. It is **discover-then-plan-then-apply**: discovery validates the
target and resolves its framework once; planning reads only; the command prints and confirms
the plan before touching the tree.

1. `scaffolding.Discover(wd)` validates the target: it exists, is a directory, and is its
   own git repo root
   (`git.IsRoot` — a `.git` **directly** in `wd`, no walk-up, so a standalone repo nested
   inside another checkout counts only with its own `.git`).
2. The same call resolves the owning framework and returns a `Target` that retains it. The
   command prompts for the additional inputs `Target.ScaffoldVars()` reports.
3. `Target.Plan(info, vars)` calls the retained framework's `Scaffold` once. Its
   `scaffold.Spec` carries the module, vars, rocks, resolved files, and seeded `strategy`;
   the target computes the `platform.toml` disposition and returns a read-only plan.
4. The command prints and confirms the plan, then calls `Plan.Apply()` or, for `--force`,
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
  versions to cut), the seeded default `[vars]`, and the `[rocks]` pin.
- **Present** → the surgical merge (below) folds the default `[vars]` and the `[rocks]`
  pin in **textually**; every other table, comment, and byte is preserved.

### Re-init: surgical merge

Planning a re-init computes the surgical merge below. Applying the plan while keeping
existing files leaves `platform.toml` untouched, so the planned additions are written only
when replacement is accepted (`--force` or the overwrite confirmation). Replacement uses
the merged bytes, preserving the operator's existing config.

The merge (owned by `conf/`, alongside `Generate`) folds the defaults into `[vars]` and
`[rocks]` **line-by-line, not by decode/re-encode** — a round-trip through the TOML
encoder would lose the operator's comments, ordering, and formatting. For each table, the merge:

- **Appends** default keys the file lacks, inserted after the last non-blank line of the
  existing table body (or a fresh table appended to EOF when absent).
- **Preserves** keys already present — the operator's value stands untouched, so an
  existing `[rocks]` pin is never bumped by re-init.
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
  the rock's; a newer rock changes nothing in a repo until its files are edited or
  re-copied.
- **No baseline is embedded in `platform`.** The baseline's source of truth is the rock's
  `templates/`; `framework/skel` holds only the launcher.
