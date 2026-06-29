# ADR: Monorepo layout + SvelteKit (JS) UI

- **Status:** Superseded (2026-06-29) — see banner
- **Date:** 2026-06-14
- **From:** 2026-06 platformv2 design walk

> **Superseded (2026-06-29):** the `core/` + `api/`/`cli/` monorepo restructure and the
> single-module-spanning-`api/cli/core` layout are dropped. platform stays **one Go module
> with flat top-level packages** (no `core/` grab-bag); the server ships in-binary as
> `platform serve` (+ a future `srv/` package). See
> [`../notes/2026-06-29-platform-as-ci-design.md`](../notes/2026-06-29-platform-as-ci-design.md).
> The **SvelteKit-in-plain-JS UI** and **multi-call OpenTofu-provider-in-the-CLI** decisions
> still stand.

## Context

platformv2 grows from a single Go CLI into server + CLI + web + shared logic. We need a
layout, a UI stack (TypeScript is banned), and a home for the OpenTofu provider.

## Decision

Restructure to a **monorepo**: `api/` (Go server), `cli/` (Go CLI), `ui/` (SvelteKit), and
`core/` (shared Go: builder, project, releases, gitctx, api-client, types) — **one Go
module** spanning `api/cli/core`. The UI is **SvelteKit in plain JS** (Svelte's JS mode +
JSDoc), built with **adapter-static** and `go:embed`'d into the `api` binary (one server
binary at runtime, no node in prod). The **OpenTofu provider is the CLI binary**
(multi-call: argv[0] `terraform-provider-platform` → gRPC provider; else CLI), reusing the
CLI's API client + `platform login` token.

## Alternatives rejected

- **Per-component Go modules** — version-sync friction; `api`/`cli` share types.
- **Go templates + htmx** (Claude's earlier inference) — fine, but the call is SvelteKit.
- **TypeScript / a separate provider repo+release** — TS is banned; the multi-call binary
  removes a whole release pipeline.
- **adapter-node SSR** — adds a node runtime; not needed.

## Consequences

- The restructure touches the test harness (`test.sh`/`tests.cue`/testbeds), Dockerfile, and
  the bootstrapper templates — migrate incrementally, not big-bang.
- `platform tf install` writes a `dev_overrides` entry; no registry/signing.
