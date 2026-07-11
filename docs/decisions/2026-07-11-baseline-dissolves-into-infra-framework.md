# Baseline installs unconditionally; the baseline dissolves into the Infra framework

- **Date:** 2026-07-11
- **PR:** manual
- **Status:** accepted (supersedes the install-time **picker** half of
  [2026-06-22 flat-baseline-install-time-selection](2026-06-22-flat-baseline-install-time-selection.md);
  that ADR's destination-encoding / no-marker-grammar / pins-only half stands)

## Decision

Under the Framework refactor a `Framework` is the sole owner of a project type, so the
cluster baseline — the thing an infra repo scaffolds — belongs to the **`Infra` framework**,
not to a standalone package or an interactive picker.

1. **No install-time picker.** `platform init` no longer shows an `OptionalMultiSelect` of
   components with a `Defaults`-vs-`Mandatory` split. The full default baseline installs
   **unconditionally** on an infra init. Selection is not an operator choice at init time;
   an operator prunes what they don't want by editing the committed repo afterward.
2. **The `baseline/` package dissolves.** Its text/template rendering merges into the single
   `framework/scaffold/` mechanism (one templating mechanism, not two). Its embedded files,
   version pins, and destination routing move **into the `Infra` framework**, which owns them
   as the assets it scaffolds.
3. **Registry creds are defaulted, not prompted.** The baseline ships `#registry_username` /
   `#registry_password` as empty placeholders in committed CUE that the operator hand-edits —
   consistent with the committed-literal model. `init` prompts for neither.

What the 2026-06-22 ADR established and **still holds**: the baseline is destination-encoded
by name (`apps-*` → `apps/`, `defaults-*` → `defaults/`, root → repo root); there is no
filename marker grammar and no render-time gating; `[ops.vars]` carries version pins only;
`render` applies whatever is present, routing by extension.

## Rationale

The picker was selection machinery for a choice that isn't the operator's to make at init
time. The baseline is an opinionated appliance (see
[2026-06-17 opinionated-appliance-embedded-init](2026-06-17-opinionated-appliance-embedded-init.md)):
its default working set is what a fresh infra repo *is*, and an operator who wants less edits
the committed result — the same hand-edit model already governing the image ref and the CUE
literals. A picker adds an interactive decision, a `Defaults`/`Mandatory` split, and
`selectComponents` glue to defer a choice that a convention covers for free.

Relocating the baseline into the `Infra` framework follows the refactor's central principle:
the framework is the sole source of truth for its project type. A separate `baseline/`
package holding infra's files, plus a separate templating path in `baseline.Render`, split
that ownership across three places and duplicated the file/template mechanism. One mechanism
(`framework/scaffold/`), one owner (`Infra`).

## Consequences

- `init` has no component-selection surface; `selectComponents`, `OptionalMultiSelect`, and
  the `baseline.Defaults`/`baseline.Mandatory` split are removed.
- There is exactly one files/templating mechanism, `framework/scaffold/`.
- `Infra.Scaffold` produces the baseline (its files, holes unresolved; its default `[ops.vars]`
  pins; its `strategy="latest"` seed) — there is no app-vs-infra predicate or branch; the
  distinction is pure `Scaffold` polymorphism.
- An operator who wants a smaller baseline prunes the committed files after init.
