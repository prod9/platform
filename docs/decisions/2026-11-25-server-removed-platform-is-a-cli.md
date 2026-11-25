# The server is removed; `platform` is a command-line tool

- **Date:** 2026-11-25
- **PR:** manual
- **Status:** accepted
- **Supersedes:**
  - [2026-06-14 — Platform in-cluster control plane](2026-06-14-platform-in-cluster-control-plane.md).
  - [2026-06-14 — Identity and linked accounts](2026-06-14-identity-and-linked-accounts.md).
  - [2026-06-14 — Monorepo and Svelte UI](2026-06-14-monorepo-and-svelte-ui.md): the halves
    that still stood, the SvelteKit UI and the OpenTofu provider built on the server API
    and `platform login`.
  - [2026-06-14 — Secrets: platform-pull](2026-06-14-secrets-platform-pull.md): secret
    values lived in the server.
  - [2026-06-21 — Dagger engine: StatefulSet + TCP](2026-06-21-dagger-engine-statefulset-tcp.md):
    the in-cluster engine pool served the server's builds and is no longer scaffolded.
  - [2026-06-24 — Split build and server log channels](2026-06-24-split-build-and-server-log-channels.md):
    the server channel. The build console channel stands.
  - [2026-06-29 — Platform server: GitHub App, zero platform RBAC](2026-06-29-platform-server-github-app-zero-rbac.md).
  - [2026-08-24 — Execution mode does not define delivery policy](2026-08-24-execution-mode-does-not-define-delivery-policy.md):
    the server execution mode and its delivery policy. Its rulings that `release` and
    `publish` are orthogonal and that there is no `deploy` verb stand.

## Decision

`platform` has no server. `srv/` with its migrations, `webui/`, the `srv` and `worker`
commands, the `[server]` config, the engine's source checkout and inspection (called only
by the server), the scaffold template that deploys the server and its engine pool, and the
Go dependencies nothing else needs are removed. Tag `v0.10.0` is the last commit with the
server; the code stays reachable there.

The build chain is untouched: commands, build frameworks, Dagger use, `release`,
`publish` and every other `platform.toml` key work as before, as does the engine's client
code for a configured remote Dagger engine.

chakrit ruled it: "rip out ALL of platform's server components, all of it, I give up, it's
too complicated. Keep it in git history or archive branch is fine." On the scope: "why the
hell is the build command/chain impacted at all to begin with? i said to just remove the
server part?"

## Rationale

The server was most of the code and most of the open design, and it carried the CLI's
release line along with it: a web application on fx with published migrations, GitHub App
identity, a worker, checkout caches and an engine pool, to provide CI/CD that a CLI plus
Flux already delivers. Every record it superseded served that server, so they go with it
rather than linger as rulings over code that no longer exists.

The obvious smaller move is to freeze the server and keep it in the tree. That keeps its
dependencies, its laws (fx conventions, immutable published migrations) and its specs
binding on every change to the CLI. Git history keeps the work at `v0.10.0` without those
costs.

The build chain stays because it was never part of the problem. A local build, `release`
and `publish` share the engine with the server but not its complexity; removing the server
removes a driver, not the thing it drove.

## Consequences

- Delivery is the CLI: `publish` pushes the image, Flux pulls it. `render` plus
  `kubectl apply` remains the no-Flux path.
- Pull-based Flux delivery
  ([2026-06-14](2026-06-14-pull-based-gitops-timoni-flux.md)), the committed-image model
  ([2026-06-26](2026-06-26-render-is-pure-function-of-committed-git.md)) and the
  Dagger-session ruling ([2026-07-27](2026-07-27-a-dagger-client-is-a-session.md)) are not
  server records and stand; the session record's open question about a long-lived server
  session no longer arises.
- The platform server already running in `prod9-main` is outside this decision. The
  launcher's `go install platform.prodigy9.co@VERSION` resolves through it, so installing
  `platform` is unaffected until that deployment is retired.
- `platform`'s own `platform.toml` builds a server image (`args = ["srv"]`); what it
  publishes now is for the specs to settle.
