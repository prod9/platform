# Platform Server

Status: **target design — skeleton implemented, surface under active rework.** Read this as
intent, not settled current state: the route surface, the setup/install flow, the boot
sequence, and the flux-webhook half are all being re-decided, and nothing here is closed
until that lands. The implementation notes below are accurate to the code as it stands.

> **Target design — skeleton implemented.** A `srv/` tree now exists: the router +
> `platform serve` command and the embedded `webui/` seam (placeholder page,
> `GET /api/health`) + DB (users/identities per the
> [identity ADR](../decisions/2026-06-14-identity-and-linked-accounts.md)), migrations
> embedded in `srv/`, run at boot. The GitHub App bootstrap is implemented: the manifest
> flow lives at `/setup/github` (manifest form) + `/setup/github/callback` (code
> exchange), storing the App credentials encrypted in the single-row `github_app` table.
> Webhook ingest is implemented: `POST /api/webhooks/github` verifies the App webhook
> HMAC signature and records a queued `builds` row for each pushed version tag
> (`refs/tags/v*`, not deleted); the build runner below consumes the queue.
> Repo-prep is implemented (`srv/builds/repoprep.go`): `PrepRepo` maintains the full bare
> mirror under a per-repo flock, resolves the sha, and adds the per-build worktree
> (§Repo preparation below); `RemoveWorkTree` is the post-build cleanup; cache root via
> `CACHE_DIR` (default `/var/cache/platform`). Engine wiring is implemented
> (`srv/builds/builds.go` + `srv/builds/runner.go`): `Serve` opens one `engine.New` per process and a
> claim loop consumes queued builds — `ClaimBuild` (`FOR UPDATE SKIP LOCKED`, oldest
> first) → repo-prep → `conf.Load` → `engine.BuildAndPublish` under the build's tag →
> `FinishBuild`/`FailBuild` records the outcome (2s poll tick when the queue is empty).
> GitHub login is implemented (`srv/auth/auth.go`): `/api/auth/github` +
> `/api/auth/github/callback` run the App's user-OAuth flow, find-or-create
> user+identity per the [identity ADR](../decisions/2026-06-14-identity-and-linked-accounts.md)
> (user token encrypted into identity metadata; no refresh handling and no
> verified-email auto-link yet), and mint a platform session — a random token whose
> SHA-256 lands in the `sessions` table, carried by a 30-day `platform_session`
> cookie, revoked by `POST /api/auth/logout`. The web-UI API is implemented: `GET /api/me`
> (`srv/auth/auth.go`) and `GET /api/builds` (`srv/builds/api.go`) authenticate against
> that session (hand-written wire structs — see §No `api/` contract layer).
> Installation-token minting is implemented (`srv/github/tokens.go`): a hand-rolled
> RS256 App JWT resolves the repo's installation and mints its short-lived token
> (§Two token types). **VOID (2026-07-18):** its implemented consumer
> `POST /api/repos/{owner}/{repo}/flux-webhook` (`srv/flux/webhook.go`) is dead — the
> GitHub→Flux `registry_package` webhook is **org-wide, provisioned once in the install
> flow**, never minted per-repo. The endpoint drops in the srv rebuild.
> `srv` is the **second driver** of the one-publish-engine model — the tag-watch
> server invoking the same build+push engine the local CLI drives (see
> [delivery-verbs-are-orthogonal](../decisions/2026-07-05-delivery-verbs-are-orthogonal.md)
> and the one-engine-two-drivers model in [engine.md](engine.md)). The frozen ruling
> behind the auth model lives in
> [platform-server-github-app-zero-rbac](../decisions/2026-06-29-platform-server-github-app-zero-rbac.md).
> Source:
> [platform-as-CI design (prior-art)](../scratch/prior-art.md#platform-as-ci-architecture-design-2026-06-29).

## What `srv` is

`srv` is the API + webhook processor: on a push it clones the repo, builds the image,
renders + publishes the infra artifact, and lets Flux pull it. It owns the GitHub App, the
DB, and token minting. It is a layer above the **shared packages** (the stateless
build/render/publish/release machinery: `framework`, `engine`, `gitops`, `releases`, …)
and consumes them per request — the engine is an `sql.DB`-style, context-carried fleet
handle (`engine.New(cfg)` once, `engine.Build(ctx, …)` per call) so a long-running server
can reuse it.

`srv` ships **in the same binary** as the CLI — `platform serve` starts the process. One
Go module (`platform.prodigy9.co`); the shared packages, `cmd`, and `srv` are conceptual
layers (flat packages at the repo root — no `core/` grab-bag, see
[architecture.md](architecture.md)), not separate `go.mod`s. The dependency rule is
one-directional, guarded by a boundary test (`srv/boundary_test.go`): **the shared
packages are the leaves and must never import server
concerns** — no `fx/data`/`sqlx`/migrations, no `net/http` server, no auth, no knowledge
that `srv` exists.

Internally `srv` is organized as **self-contained fx-style fragments** — one subpackage
per concern (`srv/auth`, `srv/github`, `srv/builds`, `srv/flux`), each carrying its own
domain models, controllers, and embedded migration SQL, portable by copy-paste. The root
package composes them: `Router` mounts every fragment's controllers; `Serve` aggregates
their `Migrations` embeds at boot (`srv/migrate.Merged`, timestamps re-sorted across
fragments). The fragment import graph is acyclic — `auth → github`,
`builds → {auth, github}`, `flux → {auth, github}` — with `flux` split out precisely so
github (which auth imports) never imports auth back. `srv/pgerr` and `srv/srvtest` hold
the shared postgres-error check and fragment-neutral test scaffolding.

### Operations (current surface)

Everything the running server does today — the review/grill table. HTTP first, then the
boot/background operations that run without a request.

| Operation                                       | Gate                          | What it does                                                                          | Why it exists                                                                                              |
| ----------------------------------------------- | ----------------------------- | ------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `GET /api/health`                               | none                          | `{"time": …}` liveness probe                                                           | k8s probes + smoke-level "is the server up" check without touching DB or auth                               |
| `GET /api/auth/github`                          | none                          | sets the state cookie, redirects to GitHub's user-OAuth authorize page                 | login entry point — platform delegates identity to GitHub, holds no passwords (identity ADR)                |
| `GET /api/auth/github/callback`                 | state cookie                  | exchanges the code, `GET /user`, find-or-create user+identity, mints a session cookie  | completes login; identity keyed on immutable provider id so GitHub renames don't break links (identity ADR) |
| `POST /api/auth/logout`                         | none (cookie optional)        | deletes the session row, clears the cookie                                             | session revocation server-side — a stolen cookie dies with the row, not with the browser                     |
| `GET /api/me`                                   | session                       | id + name of the session's user                                                        | the webui's "who am I / am I logged in" probe                                                                |
| `GET /api/builds`                               | session                       | last 50 builds, newest first                                                           | the webui's build list — the server's whole point made visible                                               |
| `POST /api/webhooks/github`                     | App webhook HMAC              | verifies signature; queues a build row per pushed `refs/tags/v*`                       | the pull-model trigger: a version tag *is* the build request (delivery-verbs ADR)                            |
| `GET /setup/github` (+ `/callback`)             | none (operator bootstrap)     | App Manifest form; callback stores exchanged App credentials (single row, encrypted)   | one-time App bootstrap without hand-copying secrets; a second App is a hard 409                              |
| `GET /*`                                        | none                          | serves the embedded webui (`webui.Assets`)                                             | single-binary delivery — no separate frontend deploy                                                         |

Boot/background, in `Serve` order:

| Operation             | When            | What it does                                                                    | Why it exists                                                                                 |
| --------------------- | --------------- | -------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| DB connect + migrate  | boot, fail-fast | requires `DATABASE_URL`; applies embedded SQL; refuses a dirty migration state    | schema drift is an operator decision, never silently resynced                                     |
| requeue orphan builds | boot            | flips every `running` row back to `queued`                                        | single-server model: a running row at boot can only be a crashed predecessor's orphan             |
| build runner loop     | continuous      | claim (`SKIP LOCKED`) → mirror+worktree prep → `conf.Load` → `BuildAndPublish` → finish/fail; 2s poll, immediate re-claim | the server driver of the one-publish-engine model — same `BuildAndPublish` the CLI drives (delivery-verbs ADR) |

### No `api/` contract layer (deliberate)

A shared `api/` package of wire types + generated client is **rejected as over-engineering**
at this stage: it earns its keep only with *independent*, *public/versioned*, or *polyglot*
consumers — none true for an internal, single-consumer, Go-to-Go tool with no backward-compat
obligation. When the CLI eventually calls `srv`, it carries its own small **hand-written
client structs**, kept in step with the handlers by hand; the cost (a few duplicated structs,
contract drift surfacing at runtime not compile time) is acceptable at this surface size. The
hard rule: **`cli` must not import `srv`** — that would drag the server's DB and transitive
deps into the CLI binary; `cli` stays shared-packages + stdlib `net/http` only. A
contract/codegen layer returns to the table only when a real second consumer appears (a
non-Go `webui`, or external API users), i.e. when versioning actually bites.

## Authorization: delegate to GitHub, zero platform RBAC

Platform stores **no permission tables and configures no roles**. Authorization is
whatever GitHub already says:

- A user who can access the repo can trigger its builds.
- Deploy permission is whether that user can write to the infra repo.

This is mechanically clean because **a deploy *is* a commit to the infra repo** (the
committed image-literal model — see
[render-is-pure-function-of-committed-git](../decisions/2026-06-26-render-is-pure-function-of-committed-git.md)).
GitHub's write bit on the infra repo *is* the deploy gate, with nothing to configure. The
consequence for credentials: platform must act with the **triggering user's GitHub
identity** where attribution/gating matters, never a single god credential that would
force platform to decide who-can-do-what.

## Auth mechanism: a GitHub App

`platform` authenticates as a **GitHub App** — the GitHub-sanctioned integration model
(the path GitHub Actions, Vercel, Jenkins, post-migration CircleCI, and Buildkite's
control plane use). Chosen over an OAuth App because it removes the two failure modes an
OAuth-token approach forces you to work around: a stored long-lived per-user secret, and a
bus-factor on whoever connected the repo.

### `srv` owns the App

The server owns the App and creates it **once, at server setup**, via GitHub's **App
Manifest flow**: `srv` generates a manifest (permissions `contents:rw`, `metadata:r`;
webhook events incl. `push`; webhook + callback URLs), the operator clicks **Create GitHub
App** on GitHub, and GitHub redirects back with a one-time code that `srv` exchanges
(`POST /app-manifests/{code}/conversions`) to receive the **app id, private key, webhook
secret, client secret** automatically. No manual "copy the private key into config." This
is a *server-bootstrap* step — **not** `platform init`.

### Two token types, chosen per operation

| Token                  | Identity            | Scope                                        | Used for                                        |
| ---------------------- | ------------------- | -------------------------------------------- | ----------------------------------------------- |
| **Installation token** | `platform[bot]`     | installed repos ∩ granted permissions, ~1h   | webhook-driven / autonomous work (clone, build, publish) |
| **User-to-server**     | the triggering user | (user's access) ∩ (app's granted perms)      | where attribution + per-user gating matter (a deploy) |

- **Installation token** — minted from the app key (JWT → installation), app/bot identity,
  short-lived. No bus-factor; commits attributed to `platform[bot]`.
- **User-to-server token** — obtained via the App's user OAuth flow, acts as the user.
  Used where the infra-repo commit must show as the user and be gated by *their* write
  access. It restores implicit authz (the token can't exceed the user's reach), so the
  explicit "does user X have access" API check is only needed on the installation-token
  path.

### Constraints to design around

- **Install is required.** Either token only reaches a repo where the App is **installed**
  (and, for the user token, where the user *also* has access). Unlike a raw OAuth token, a
  GitHub App user token cannot reach every repo the user can — the install is the gate,
  and is also what enables webhooks. Accepted trade.
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

## Repo preparation (CI clones)

Cloning is **not** part of any framework's build phase. On a server run there is no local
checkout, so a dedicated **repo-prep phase** (in `srv`, above the shared packages) produces a local
working tree and hands its path to the *unchanged* build machinery — already
parameterized by working dir (`conf.Load(wd)`, `host.Directory(unit.WorkDir)`).
Local and CI runs then take the identical build path; a local run simply has no prep
phase ("you're already in the dir").

```
local:  Load(".")                      → AttemptFrom → engine.Build
CI:     repo-prep: clone url@sha → <wd>      → Load(wd) → AttemptFrom → engine.Build
                                               └──────── identical from here ────────┘
```

Clones are plain `git` to local fs — no dagger needed for sourcing, so the in-process CUE
render and `host.Directory` both work directly against the clone. repo-prep also returns
the **resolved sha** so the committed-image-pin model has its anchor.

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
the mirror cache makes full clones cheap, so shallow buys nothing.

## Sequencing

Each layer consumes the one below *after* it works:

1. **Prove the delivery path from the CLI** end-to-end — the `Infra` framework → render →
   publish → Flux pulls → applies. All shared-package work, no server.
2. **Wrap it in `srv`** — webhook ingest + GitHub App + token store + the API.
   Orchestration around a proven path.
3. **`webui`** on top of a proven API.

The framework refactor + the `Infra` framework are shared-package work and proceed regardless of
the server timeline — none of the server/auth design gates the next coding step.

## Open details (not blockers)

- Where the `init` server marker lives — `platform.toml` `[server]` field vs CLI-global
  config.
- User-token expiry policy — current default: the user token is stored as received
  with no refresh handling (pair it with the App's non-expiring setting); the platform
  session lasts 30 days. Expiring tokens + refresh return here if the balance shifts.
