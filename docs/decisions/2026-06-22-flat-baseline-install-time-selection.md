# Flat baseline: install-time selection, no marker grammar

- **Date:** 2026-06-22
- **PR:** manual
- **Status:** partly superseded (supersedes the marker grammar + render-time gating from
  D3b-2; naming + destinations amended 2026-07-06 for the three-destination
  root/`apps/`/`defaults/` model). The install-time **picker** half is superseded by
  [2026-07-11 baseline-dissolves-into-infra-framework](2026-07-11-baseline-dissolves-into-infra-framework.md)
  — the baseline now installs unconditionally and moves into the `Infra` framework. The
  destination-encoding / no-marker-grammar / pins-only half below still stands.

## Decision

The embedded cluster baseline is **one flat list** of component files (`core/baseline/files/*`
— both `.platform` directives and `.cue` apps, **destination-encoded by name**) plus a
**hard-coded `Defaults`** list. `platform init` shows the whole list with `Defaults`
pre-checked; each chosen file installs to the destination its name encodes — `apps-*` → the
target repo's `apps/`, `defaults-*` → `defaults/`, root files (e.g. `platform.toml`) → the repo
root. `apps/` holds **only render-able** components (each top-level key becomes an `ops render`
output); shared definitions (e.g. `#Basics`) live in the **mandatory** `defaults/` package,
imported by `apps/`. `ops render` applies **whatever is present** (route by extension). There is
**no** filename marker grammar (`@variant`, `+flag`), **no** `baseline.Select`/`ScanOptions`,
and **no** render-time gating on `[ops.vars]`. `[ops.vars]` carries version pins only — selection
is not a var. (The destination prefix is a routing key, not a selection marker — orthogonal to
the grammar rejected above.)

## Rationale

D3b-2 encoded component selection as filename markers (`name@variant.platform`,
`name+flag.platform`) resolved at render time by `baseline.Select` against `[ops.vars]`. In
practice that was over-built for what it bought:

- **The markers were noise.** `argocd+argocd.platform` (component == flag) and
  `nginx-gateway@experimental.platform` read badly, and the grammar (`parse`, `entry`,
  `Option`, `resolveChoices`) plus `Select`/`ScanOptions` was a lot of code to express "which
  files do you want."
- **Render-time gating earned nothing.** Operators change their baseline by **re-running
  init** (`re-init → select stable → bump the version`), not by toggling a var and
  re-rendering. So the gate belongs at install time — pick the files once, write them — not at
  every render.
- **No mutual-exclusion machinery needed.** A "choice" (experimental vs stable nginx-gateway)
  is just two files in the list with one in `Defaults`. If an operator keeps both, that's their
  call — we don't police it.

So selection collapses to: derive the list from the built-in files, hard-code the `Defaults`,
let the picker do the rest. That is the whole of the logic.

Versions still gate nothing but still matter: directives interpolate `\(var)` from `[ops.vars]`
into `download` URLs, so `DefaultVars` keeps the pins (`cert_manager_version`, …) and drops the
selection toggles (`argocd`, `ngf_experimental`).

## Consequences

- Deleted `core/baseline/options.go` (`Select`/`ScanOptions`/`parse`/`Option`/markers).
  `core/gitops/render.go` no longer calls `Select`; its two routes are renamed for the new
  model — `renderDirectives` applies every `.platform` found, `renderCue` exports the `.cue`
  files (skipped when there are none). Both read the same co-located `apps/` dir; the
  directive→output-dir mapping moved here as `outputName` (baseline no longer owns it).
- Files are **destination-encoded by name** (`apps-*`, `defaults-*`, root); `nginx-gateway.platform`
  (stable, `standard-install`) sits alongside `nginx-gateway-experimental.platform` for the
  TCPRoute-graduates future.
- Render-able `.platform` and `.cue` co-locate in the target's `apps/` (route by extension);
  shared definitions live in `defaults/`, imported by `apps/`. The separate `baseline/` dir is
  gone.
- The init picker is one `prompts.OptionalMultiSelect(question, Defaults, allFiles)`.
- **Do not re-introduce filename markers or render-time selection gating.** If a future need
  looks like it wants them, prefer another flat-list + Defaults pass first.
</content>
