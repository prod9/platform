# Flat baseline: install-time selection, no marker grammar

- **Date:** 2026-06-22
- **PR:** manual
- **Status:** accepted (supersedes the marker grammar + render-time gating from D3b-2)

## Decision

The embedded cluster baseline is **one flat list** of component files (`core/baseline/files/*`
— both `.platform` directives and `.cue` apps, with clean names) plus a **hard-coded
`Defaults`** list. `platform init` shows the whole list with `Defaults` pre-checked; the
operator's chosen subset is written into the target repo's `apps/`. `ops render` applies
**whatever is present** (route by extension). There is **no** filename marker grammar
(`@variant`, `+flag`), **no** `baseline.Select`/`ScanOptions`, and **no** render-time gating
on `[ops.vars]`. `[ops.vars]` carries version pins only — selection is not a var.

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
  `core/gitops/render.go` no longer calls `Select`; `renderBaseline` applies every `.platform`
  found, and `renderApps` skips when there are no `.cue` files (both routes read the same
  co-located `apps/` dir).
- Files renamed to clean names; `nginx-gateway.platform` (stable, `standard-install`) added
  alongside `nginx-gateway-experimental.platform` for the TCPRoute-graduates future.
- `.platform` and `.cue` co-locate in the target's `apps/` (one source tree, route by
  extension) — the separate `baseline/` dir is gone.
- The init picker is one `prompts.OptionalMultiSelect(question, Defaults, allFiles)`.
- **Do not re-introduce filename markers or render-time selection gating.** If a future need
  looks like it wants them, prefer another flat-list + Defaults pass first.
</content>
