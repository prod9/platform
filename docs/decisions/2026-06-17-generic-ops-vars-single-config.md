# DSL vars are a generic open map in platform.toml; settings.toml is eliminated

- **Status:** accepted
- **Date:** 2026-06-17
- **From:** Slice 1 close-out discussion (chakrit)

## Context

The manifest patch DSL needs values for its `${var}` substitution — chiefly upstream
version pins (cert-manager, NGF, …) and gating flags. infra-cli stored these in a
`settings.toml` with a **typed** struct (`settings.Settings`: `CertManager.Version`,
`NginxGateway.{Version,GatewayAPIVersion,Experimental,…}`). Porting that struct as-is
would tie the platform config DTO to a fixed set of software: every new component or knob
the DSL references would force a second edit to the Go struct plus a recompile, and would
split config across two per-repo files (`platform.toml` + `settings.toml`).

## Decision

1. **Eliminate `settings.toml`.** All per-repo config lives in `platform.toml`.
2. **`[ops.vars]` is a generic open `map[string]string`.** The config processor stores it
   verbatim — no per-software fields, no typed component structs. The DSL owns its own
   variable vocabulary; adding/removing a `${var}` means editing the directive file and
   the `[ops.vars]` table, never the Go DTO.
3. **Values are strings.** Bools and numbers are strings in TOML
   (`experimental = "true"`); the per-component assembly layer interprets them (e.g. gates
   a directive line on `vars["nginx_experimental"] == "true"`). This keeps substitution
   type-free and the processor a pure passthrough.
4. **`[ops].image`/`tag` stay typed.** The publish target is platform's own concern, not a
   DSL var — it remains structured config, distinct from the generic `[ops.vars]` bag.

## Alternatives rejected

- **Typed per-component struct (port settings.Settings).** The Go default, and what a
  future agent will be tempted to "fix" this map into. Rejected: it recouples the config
  schema to the DSL's vocabulary — the exact churn this decision removes.
- **Two files (`platform.toml` + `settings.toml`).** Rejected: one source of truth per
  repo; no reason to keep a second config format alive past the port.

## Consequences

- The processor (`project.Ops`) gains `Vars map[string]string` and nothing
  software-specific.
  Validation of var *names* moves to render time (a missing `${var}` fails loudly); a
  directive-level `requires` can later make that a clean, early error.
- **Migration (D3):** the existing infra-repo `settings.toml` content moves into
  `platform.toml` — version/flag fields → `[ops.vars]`; overlapping fields (`maintainers`,
  `repo.url`) collapse into platform.toml's existing `maintainer`/`repository`. Richer
  shapes (the NGF `annotations` map) flatten to individual vars or are handled by the
  directive that consumes them; the general nested case is deferred.
- Supersedes open-question #10 (var source) and its two-files sub-question.
