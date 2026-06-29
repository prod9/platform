# Platform-as-CI — architecture design

> Status: **design, not implemented.** This note records the target architecture for
> turning `platform` from a local CLI into a self-hosted CI/CD service (webhook-driven
> builds + GitOps delivery) while keeping the existing CLI working. Nothing here is built
> yet; it exists so the module boundaries and the auth model are settled *before* the
> server work starts. Companion: the infra-builder shape lives in
> [`2026-06-29-builders-reshape-design.md`](2026-06-29-builders-reshape-design.md).

## Context

Today `platform` is a single-module Go CLI (`platform.prodigy9.co`) that builds containers
via Dagger, renders GitOps manifests (`ops render`/`gitops`), and publishes OCI artifacts
— all as **local** operations the operator runs by hand. The engine was already reshaped
into an `sql.DB`-style, context-carried fleet handle (`engine.New(cfg)` once,
`engine.Build(ctx, …)` per call) specifically so a long-running server can reuse it per
request.

The goal of this design is the server that consumes that engine: a webhook handler that,
on a push, clones the repo, builds the image, renders + publishes the infra artifact, and
lets Flux pull it — with **GitHub as the sole source of authorization** and **no
platform-side RBAC**.

## Module architecture

One Go module; `core`/`cli`/`srv` are conceptual layers (packages at the repo root), not
separate `go.mod`s. Four roles — two now, two later:

| Layer      | Role                                                           | Depends on       | When   |
|------------|---------------------------------------------------------------|------------------|--------|
| **core**   | Stateless engine — build / render / publish / release. No DB, no HTTP, no auth. | — (leaf) | now    |
| **cli**    | The `platform` command — local builds + (later) repo onboarding. | core           | now    |
| **srv**    | API + webhook processor. Owns the GitHub App, the DB, token minting. | core         | future |
| **webui**  | Frontend over `srv`'s API.                                    | srv's API (wire) | future |

### Single module, not a workspace

The server ships **in the same binary** as the CLI — `platform serve` starts the webhook/API
process. That collapses the case for a `go.work` multi-module split: the headline benefit of a
separate `core` `go.mod` was dependency-set minimization (core's deps can't include the CLI's
`cobra`/`pterm` or the server's DB driver), but that only buys anything across *separate
binaries*. A co-binary links the union regardless — so the split would carry the nested-module
tagging tax (`go install @latest` needs coordinated `core/vX.Y.Z` tags) for no offsetting win.

So: **one module, `platform.prodigy9.co`.** No `go.work`, no `core/` umbrella directory (a
folder whose only job is to be a module root earns nothing once it isn't one — library
packages stay at the repo root). `cmd/` is unchanged. The accepted trade: `go install` of the
CLI pulls the server's transitive deps (DB driver, `net/http`) into the laptop binary — small
next to dagger, dormant unless `serve` runs. If it ever bites, splitting `srv` into its own
binary is a later additive move, not a pre-payment.

### The dependency rule (still binding, enforced by lint)

**`core` is the leaf and must never import server concerns.** No `fx/data`/`sqlx`/
migrations, no `net/http` server, no auth, no knowledge that `srv` exists. `core` is
consumed *by* `srv`; it must not reach back. Without the module boundary this is enforced by
an **import-graph check in `test.sh`** — introduced when `srv/` actually lands (the library
packages must not import `cmd/` or `srv/`). Writing it now, against a tree with no `srv/`,
guards nothing.

### No `api/` contract layer (deliberate)

A shared `api/` package of wire types + generated client was considered and **rejected as
over-engineering** for this stage. It earns its keep only with *independent* consumers, a
*public/versioned* surface, or *polyglot* clients forcing a schema — none of which hold
for an internal, single-consumer, Go-to-Go tool with no backward-compat obligations.

Instead: when the CLI eventually needs to call the server, it carries its **own small,
hand-written client structs**, kept in step with `srv`'s handlers by hand. The cost — a
few duplicated structs, and contract drift surfacing at runtime/integration rather than
compile time — is acceptable at this surface size. The hard rule: **`cli` must not import
`srv`** (that would drag the server's DB/transitive deps into the CLI binary); `cli` stays
`core` + stdlib `net/http` only. A shared contract/codegen layer returns to the table only
when a real second consumer appears (a non-Go `webui`, or external API users) — i.e. when
versioning actually bites.

### What the CLI needs *right now*: only `core`

Every current subcommand (`build`, `publish`, `deploy`, `ops render/publish`, `preview`,
`release`, …) is a **local** operation over the engine. There is no server to call. So the
CLI has **zero server calls** today; the client described above is future and, even then,
starts near-empty (early `init` mostly opens a browser URL, not an RPC).

## Authorization model: delegate to GitHub, zero platform RBAC

The north star: **platform stores no permission tables and configures no roles.**
Authorization is whatever GitHub already says:

- **A user who can access the repo can trigger its builds.**
- **Deploy permission is whether that user can write to the infra repo.**

This is mechanically clean because **a deploy *is* a commit to the infra repo** (the
committed image-literal model — see the gitops package and the [committed-image
ADR](../decisions/2026-06-26-render-is-pure-function-of-committed-git.md)). So GitHub's
write bit on the infra repo *is* the deploy gate, with nothing to configure. The
consequence for credentials: platform must act with the **triggering user's GitHub
identity** where attribution/gating matters, never a single god credential that would
force platform to decide who-can-do-what.

## Auth mechanism: a GitHub App

`platform` uses a **GitHub App** (the GitHub-sanctioned model for integrations; the path
used by GitHub Actions, Vercel, Jenkins, post-migration CircleCI, and Buildkite's control
plane). Chosen over an OAuth App because it removes the two failure modes an OAuth-token
approach forces you to work around: a stored long-lived per-user secret, and a bus-factor
on whoever connected the repo.

### `srv` owns the App

The **server** owns the App and creates it **once, at server setup**, via GitHub's **App
Manifest flow**: `srv` generates a manifest (permissions: `contents:rw`, `metadata:r`,
webhook events incl. `push`; webhook + callback URLs), the operator clicks **Create GitHub
App** on GitHub, and GitHub redirects back with a one-time code that `srv` exchanges
(`POST /app-manifests/{code}/conversions`) to receive the **app id, private key, webhook
secret, client secret** automatically. No manual "copy the private key into config." This
is a *server-bootstrap* step — **not** `platform init`.

### Two token types, chosen per operation

- **Installation token** — minted from the app key (JWT → installation), app/bot identity,
  short-lived (~1h), scoped to installed repos + granted permissions. Used for
  **webhook-driven / autonomous** work (clone, build, publish). No bus-factor; commits
  attributed to `platform[bot]`.
- **User-to-server token** — obtained via the App's user OAuth flow, **acts as the user**,
  bounded by **(the user's access) ∩ (the app's granted permissions)**. Used where
  **attribution and per-user gating** matter — notably a deploy, so the infra-repo commit
  shows as the user and is gated by *their* write access. This restores implicit authz
  (the token can't exceed the user's reach), so the explicit "does user X have access" API
  check is only needed on the installation-token path.

### Constraints to design around

- **Install is required.** Either token only reaches a repo where the App is **installed**
  (and, for the user token, where the user *also* has access). Unlike a raw OAuth token, a
  GitHub App user token cannot reach every repo the user can — the install is the gate
  (and is also what enables webhooks). This is the accepted trade.
- **User-token expiry is configurable** — expiring (8h) + refresh, or non-expiring (an app
  setting). Choose per the security/convenience balance.
- **Secret footprint** — one app private key + webhook secret (server-side), encrypted at
  rest; *not* a token per user. This is the first long-lived secret platform holds.
- **Callback reachability** — the manifest/install/OAuth redirects need a URL the
  operator's browser can hit that routes back to the platform process: the server's own
  (tailnet/public) URL for `srv`; a temporary local listener for a pure-CLI flow (the `gh
  auth login` pattern). The app private key is shown **once** — capture it immediately.

### Onboarding: `platform init` installs, it does not create

`platform init` is **client-side onboarding only**. It reads a marker identifying which
platform server governs this repo (open detail: a `[server]` field in `platform.toml`, or
CLI-global config → e.g. `platform.some-domain.com`), then drives **installation of that
server's existing App** onto the current repo (opens
`https://github.com/apps/<app-slug>/installations/new` scoped to the repo; GitHub
redirects back with the `installation_id`, which the server records). It **creates
nothing** — the App is the server's.

### Ownership: live from GitHub, a product concept

"Who owns this repo's pipeline" is **derived live from GitHub admin permission**, not a
platform table. To claim ownership, a user proves they currently hold **admin** on the
repo; platform verifies via the API and rebinds. Because the GitHub App already eliminates
the stored-token bus-factor, ownership is no longer an *auth-recovery* mechanism — it
survives as a **product** concept (responsible owner, who can change pipeline settings),
still GitHub-derived, still zero-RBAC.

## Repo preparation (CI clones) — a phase above the builder machinery

Cloning is **not** part of any builder's build phase. On a server run there's no local
checkout, so a dedicated **repo-prep phase** (in the `srv` layer, above `core`) produces a
local working tree and hands its path to the *unchanged* builder machinery — which is
already parameterized by working dir (`project.Configure(wd)`,
`host.Directory(unit.WorkDir)`). Local and CI runs then take the identical builder path; a
local run simply has no prep phase ("you're already in the dir").

```
local:  Configure(".")                      → AttemptFrom → engine.Build
CI:     repo-prep: clone url@sha → <wd>      → Configure(wd) → AttemptFrom → engine.Build
                                               └──────── identical from here ────────┘
```

This keeps builders context-agnostic and means clones are plain `git` to local fs — no
dagger needed for sourcing, so the in-process CUE render and `host.Directory` both work
directly against the clone.

### Cache layout (`/var/cache`), full clones

Not ephemeral `/tmp` — a persistent cache for fast clones and build reuse:

```
/var/cache/platform/
  git/<owner>/<repo>.git     ← bare mirror; `git fetch` under a per-repo lock
  work/<build-id>/           ← `git worktree add` off the mirror; removed after the build
```

One **full** bare mirror per repo, updated by incremental `fetch` (cheap after the first);
each build gets a near-instant `git worktree` that shares objects and is independently
removable (concurrency-safe: lock only the mirror's fetch). **No shallow clones** —
`--depth 1` truncates history and breaks `git subtree` (used widely across these repos);
the mirror cache makes full clones cheap, so shallow buys nothing. repo-prep also returns
the **resolved sha** so the committed-image-pin model has its anchor.

## Sequencing

Each layer consumes the one below *after* it works:

1. **Prove the delivery path from the CLI** end-to-end — infra builder → render → publish
   → Flux pulls → applies. All `core`, no server.
2. **Wrap it in `srv`** — webhook ingest + GitHub App + token store + the API. Just
   orchestration around a proven path.
3. **`webui`** on top of a proven API.

The **`#4` builders reshape + the infra builder are `core` work and proceed regardless**
of the server timeline — they land on the core side of the line either way, so none of the
server/auth design gates the next coding step.

## Immediate next: nothing structural

Single module, `cmd/` unchanged — there is **no module-split coding step**. The
`core`/`cli`/`srv` layering is package discipline, enforced by the `test.sh` import lint
added alongside `srv/`. Next real coding step is the builders reshape + infra builder.

## Open details (not blockers)

- Where the `init` server marker lives — `platform.toml` `[server]` field vs CLI-global
  config.
- The within-language run-stage dedup question carried in the builders-reshape note (§
  open).
