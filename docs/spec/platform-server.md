# Platform Server

Status: **target design — skeleton implemented, rebuild pending.** The **route surface**
and the **install/boot flow** are now settled — the
[Operations](#operations-settled-surface) table below teaches the settled surface, and
[installation.md](installation.md) owns the
install model (installer fragment, `GET /api/install`, boot composition). The **build
lifecycle** is settled too — see [Build lifecycle](#build-lifecycle-event-sourced) below.
Still open, held for a design pass: the **cluster view** (reading k8s + Flux state for
pods, logs, and rollout continuity after a publish). The implementation blockquote
immediately below describes the
**pre-rebuild skeleton as the code stands today** — its routes and setup flow are
superseded by the settled surface; read the table, not the blockquote, for the target.

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
> (`srv/builds/builds.go` + `srv/builds/runner.go`): `Serve` opens one `engine.NewSession` per
> process and a
> claim loop consumes queued builds — `ClaimBuild` (`FOR UPDATE SKIP LOCKED`, oldest
> first) → repo-prep → `conf.Load` → `sess.BuildAndPublish` under the build's tag →
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

🚨 **`srv` is a web app, and it carries a web app's own conventions.** It is built on
[prod9/fx](https://fx.prodigy9.co) and its shape — fragments, controllers, actions,
background jobs, embedded migrations, how finely it splits packages — is decided by **fx
web-app convention**, not by the laws written for the CLI and the shared packages. The
package-layout rules in [architecture.md](architecture.md) govern the shared-package graph
and hold **no jurisdiction** here; citing them to settle a question about `srv`'s internals
is a category error. A reader who wants to know how a piece of `srv` should be shaped reads
fx, then this file.

`srv` is the API + webhook processor: on a push it clones the repo, builds the image,
renders + publishes the infra artifact, and lets Flux pull it. It owns the GitHub App, the
DB, and token minting. It is a layer above the **shared packages** (the stateless
build/render/publish/release machinery: `framework`, `engine`, `gitops`, `releases`, …)
and consumes them per request — the engine layer hands out a `Session`, the span its
containers stay usable for (`engine.NewSession(ctx)` once at boot, a `Run` per unit per
request), so a long-running server reuses one session across every concurrent build. Whether
a days-long session needs liveness handling for engine pods that come and go, or `srv` opens
one session per build instead, is open — see [engine.md](engine.md), §`Session` — the unit of
lifetime.

⚠️ **Two "sessions" meet in this file, and the clash is unresolved.** A **login session** is
a user's authenticated session: the `sessions` table, the `platform_session` cookie,
`auth.SessionCtr`, `GET /api/session`. An **engine session** is `engine.Session`, the span a
built container stays usable for ([engine.md](engine.md)). They share no code, no lifetime and
no table. Which one gives ground — if either — is **deferred to the srv slice**; until then
qualify every use and write neither bare.

`srv` ships **in the same binary** as the CLI — `platform srv` starts the process (`platform serve` is a back-compat alias). One
Go module (`platform.prodigy9.co`); the shared packages, `cmd`, and `srv` are conceptual
layers (flat packages at the repo root — no `core/` grab-bag, see
[architecture.md](architecture.md)), not separate `go.mod`s. The dependency rule is
one-directional, guarded by a boundary test (`srv/boundary_test.go`): **the shared
packages are the leaves and must never import server
concerns** — no `fx/data`/`sqlx`/migrations, no `net/http` server, no auth, no knowledge
that `srv` exists.

Internally `srv` is organized as **self-contained fx-style fragments** — one subpackage
per concern (`srv/auth`, `srv/github`, `srv/builds`, `srv/install`), each carrying its own
domain models and controllers (and, where it owns tables, embedded migration SQL — `github`
is config-only, no schema). The root package composes them **per install state**: boot
decides once from `install.GetState()` whether to mount the installer fragment or the
product fragments (see [installation.md](installation.md)), and aggregates every fragment's
`Migrations` embed into one merged set (`srv/migrate.Merged`, timestamps re-sorted across
fragments) — run by the installer or the CLI, **never at boot**. The fragment import graph
is acyclic — `auth → github`, `builds → {auth, github}`, `install → {github, migrate}` —
nothing imports `srv` back, and `srv/migrate` is a leaf. `srv/pgerr` and `srv/srvtest` hold
the shared postgres-error check and fragment-neutral test scaffolding.

### Operations (settled surface)

The settled HTTP surface — the review/grill table. **Reserved backend prefixes** are
`/api`, `/auth`, `/hooks`, `/health`; the webui owns everything else under `GET /*`. JSON
lives under `/api`; GitHub-facing and health routes stay bare.

| Operation                   | Gate                      | What it does                                                                          | Why it exists                                                                                                      |
|-----------------------------|---------------------------|---------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------|
| `GET /health`               | none                      | `{"time": …}` liveness probe                                                          | k8s probes + smoke-level "is the server up" check without touching DB or auth                                      |
| `GET /auth/github`          | none                      | sets the state cookie, redirects to GitHub's user-OAuth authorize page                | login entry point — platform delegates identity to GitHub, holds no passwords (identity ADR)                       |
| `GET /auth/github/callback` | state cookie              | exchanges the code, `GET /user`, find-or-create user+identity, mints a session cookie | completes login; identity keyed on immutable provider id so GitHub renames don't break links (identity ADR)        |
| `GET /api/session`          | session                   | session state — expiry + user id; 401 when none                                       | the webui's "is my session valid" probe, distinct from the user's profile                                          |
| `DELETE /api/session`       | none (cookie optional)    | deletes the session row, clears the cookie                                            | session revocation server-side — a stolen cookie dies with the row, not with the browser                           |
| `GET /api/users/me`         | session                   | the session user's profile (id + name)                                                | the webui's "who am I" — profile, not session validity                                                             |
| `GET /api/builds`           | session                   | last 50 builds, newest first                                                          | the webui's build list — the server's whole point made visible                                                     |
| `POST /hooks/github`        | App webhook HMAC          | verifies signature; queues a build row per pushed `refs/tags/v*`                      | the pull-model trigger: a version tag *is* the build request (delivery-verbs ADR)                                  |
| `GET /api/install`          | none (installer fragment) | ordered install-state list; served **only while not completely installed**            | drives the SPA installer-vs-app decision ([installation.md](installation.md)); its 404 *is* the "installed" signal |
| `GET /*`                    | none                      | serves the embedded webui; the SPA drives installer-vs-app via `GET /api/install`     | single-binary delivery — no separate frontend deploy                                                               |

`GET /api/me` splits into `GET /api/session` (validity) + `GET /api/users/me` (profile);
`POST /api/auth/logout` becomes `DELETE /api/session`; `/api/webhooks/github` becomes bare
`/hooks/github`; the `/setup/github` App-Manifest flow is **killed** (App creation is now
a by-hand, install-page-guided step — [installation.md](installation.md)). The **Flux→srv
observability** endpoint `GET /api/repos/{owner}/{repo}/flux` is **forthcoming** — its
surface and UI need a design pass and are not settled here.

Boot no longer runs the old fail-fast sequence. The server **always boots — no hard boot
deps** (a DB unreachable is an install-state error, not a boot failure); **migrations
never auto-run at boot** (installer button or `./platform srv data migrate` — see
[installation.md](installation.md)); the boot-time **requeue-orphans** action is
**removed**. Boot instead decides the API composition once from `install.GetState()`
(installer vs product fragments — [installation.md](installation.md)). The continuous
**build runner** is an event-sourced reconciler — see below. The current skeleton's
claim-loop is described in the implementation blockquote above and is superseded.

## Triggering a build

Four boundaries, each crossable in one direction only:

```
controller ─▶ recorded intent ─▶ worker ─▶ engine ─▶ events ─▶ derived state
```

**Every trigger collapses to one domain fact first.** A webui button,
a GitHub webhook, and a future CLI trigger are the same statement — *someone asked for a
build of this repo at this commit* — differing only in how they are authorized. The
controller validates the untrusted signal, authorizes it, and records that fact. If the
button path and the webhook path diverge past the controller, that is two systems.

**A controller never calls the engine.** An HTTP request's lifetime has nothing to do with
a build's; the durable record is the handoff.

**The record is the queue.** There is no queue component — a build request is simply a
record not yet dispatched; "pending" is a property of the record, not a place it lives.
This is what makes the trigger sources interchangeable: each appends the same fact.

**The record must be complete enough to act on.** The controller serializes full intent —
**which repo, at which ref, resolved to which sha** — so the worker never re-derives *what
was asked for*.

**A build is whole-repo, so the unit set is not intent.** `platform.toml`'s `[modules]`
defines the units and a build schedules all of them; which units exist is a property of the
committed tree, not a choice a trigger makes. So the controller does not carry a unit list
and the worker reads `platform.toml` from the tree it prepared — that is reading a
definition, not re-deciding a request. Repo preparation is the same in kind: fetching the
tree fulfils a decision rather than making one, so it belongs to the worker.

### The worker is a peer *process*, and the jobs live in their fragments

**Worker** is the settled name, and it is fx's: `fx.prodigy9.co/worker` supplies the entire
machinery — the poll loop, the `jobs` table, claim and status, behind
`worker.New(cfg, jobs...)` + `Start()`. Platform writes **no worker**; it writes jobs. A job
is a `worker.Interface` (`Name() string`, `Run(ctx) error`) whose own struct is the payload.

**The separation is at the process, not the package.** `worker.Start()` blocks and runs as
its own command — `platform worker`, deployed as its own process beside `platform srv`,
scaled by adding processes. The job *code* lives in the fragment that owns its domain, per
fx's self-contained-fragment convention: the build jobs are files in `srv/builds`, a session
sweep would belong to `srv/auth`. There is no central jobs package — that is the grab-bag
fx's fragment model exists to prevent. A worker loop hand-rolled *inside* an HTTP fragment is
the pre-rework mistake (`srv/builds/runner.go`), and it is torn out rather than repaired.

**The worker is general background processing**, not a build runner: recurring cleanup
sweeps, reconciliation of bad state, and anything else that must happen off the request path
are jobs too. fx's queue is one-shot, so a recurring job reschedules itself at the end of
`Run` — no cron machinery is added.

**Two jobs carry a build**, and the split is what keeps the record the queue:

| Job           | Shape                        | What it does                                                                                                                                                                                          |
| ------------- | ---------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `scan_builds` | recurring, self-rescheduling | Reads `builds` against their events, finds every build with no terminal `run_done`, and schedules a `build` job per build with `ScheduleNowIfNotExists`. Requeuing a timed-out build is the same scan. |
| `build`       | one-shot, payload = build id | Repo-prep → engine run → write `build_events`.                                                                                                                                                        |

A controller therefore never schedules a job. It appends a record; the scan turns records
into work. That is what makes the trigger sources interchangeable, and it is why an
overlapping sweep is harmless — `ScheduleNowIfNotExists` collapses duplicates.

🚨 **A job's success is not a build's success.** A job answers *did the job do its work* —
relay the instruction to the engine, observe the execution, record what happened. A build
answers *did the build succeed*, and that answer lives only in `build_events`. A build that
failed and was correctly recorded is a **successful job**. So `Run` returns an error only
when the job itself could not do its work; a failed build returns nil. Collapsing the two
vocabularies would put build state back in fx's `jobs` table, which is the mechanism's, not
the domain's.

**It is not called a "runner."** In CI vocabulary "runner" means the agent that executes
jobs — which is what an *engine* is here — and `engine/runners` already holds that name
for Dagger endpoints. Three live concepts, three distinct words: engines execute, runners are
the endpoints they live at, workers decide what to hand them.

## Build lifecycle: event-sourced

There is **no stored build `state`.** The primitive is an append-only **`BuildEvent`**
stream in a `build_events` table — the persisted form of what a run reports through its
`Observer` ([engine.md](engine.md)), which the engine itself never serializes. The worker
executes and writes events; the database *is* the channel; the webui reads it back. Nothing
subscribes to a live in-process stream across the process boundary, which is exactly why
the engine needs no late-joining observer.

Everything else is a **fold** of that stream:

| Fold                | Computed as                                                           |
| ------------------- | --------------------------------------------------------------------- |
| current state       | reduction of the build's events so far                                |
| an **attempt**      | a `Start`→terminal span within the stream                             |
| stuck / timed-out   | last-event timestamp vs the build's `platform.toml` timeout           |

**An attempt is a fold and never a table.** There is no `build_attempts` relation: a retry
re-runs the same commit, so an attempt row would carry nothing a `run_done` boundary in the
stream does not already mark.

### The two tables

```
builds                          -- one row per trigger; immutable after insert
  id            bigserial
  trigger       text            -- 'github-push' | 'webui' | 'cli' | 'retry'
  retry_of      bigint NULL     -- REFERENCES builds(id); set only when trigger = 'retry'
  user_id       bigint          -- REFERENCES users(id); the system user for a webhook trigger
  owner         text
  repo          text
  clone_url     text
  ref           text            -- 'refs/heads/main' | 'refs/tags/v1.2.3'
  sha           text            -- the commit this build builds
  created_at    timestamptz

build_events                    -- append-only; one row per engine Observer callback
  id            bigserial
  build_id      bigint          -- REFERENCES builds(id)
  kind          text            -- step_started | step_done | image_built | published | run_done
  unit          text            -- module name; '' for run-level events
  step          text            -- '' unless step-scoped
  at            timestamptz     -- the engine's own callback time, not the insert time
  error         text            -- step_done, run_done
  image         text            -- image_built, published
  hash          text            -- published only
  stdout        text            -- captured output, per step
  stderr        text
  created_at    timestamptz
```

`build_events` is a transcription of the `Observer` contract ([engine.md](engine.md)) and
nothing more — one column per callback argument, `at` preserved as the engine reported it so
elapsed time survives a slow writer. Captured `stdout`/`stderr` ride the `step_done` row
rather than a kind of their own.

**A `builds` row records who asked and what for, never how it went.** No `status`, no
`image`, no `error` column: those are the stored state this design exists to remove, and
they live in the stream. The row is written once and never updated.

**Every build has a principal, and a webhook's is the system user.** `user_id` is `NOT
NULL`: a build nobody can be named for is a record with a hole in it, and "nobody" is not
what a webhook trigger means — the App acted, on its installation's authority. So `users`
carries one seeded row whose `identities` entry is `('system', 'platform')`, and a
webhook-triggered build attributes to it. It is a **principal, not an account**: no login
flow speaks the `system` provider, so no session can ever be minted for it, and
`identities`' `UNIQUE (provider, provider_id)` makes the row single by construction rather
than by a rule someone has to enforce. `retry_of` stays nullable because absence is real
there — a first build has no parent, and `0` would be a foreign key pointing at nothing.

**`ref` is a moving pointer, and that is the point.** A trigger names a ref — a branch or a
tag — and what a ref points at changes. `sha` is what it resolved to for *this* build, so the
committed-image model keeps its anchor ([render-is-pure-function-of-committed-git](../decisions/2026-06-26-render-is-pure-function-of-committed-git.md)),
while `ref` is the **grouping key the UI reads**: a developer watching `refs/heads/topic`
sees the failed build, the fix-push, and the green build as one list. A new push is a new
build row, never a mutation of the old one.

**A retry is a new row, linked.** Clicking retry appends a build with `trigger = 'retry'` and
`retry_of` pointing at the row it re-runs — the immediate parent, so a chain stays walkable —
and it re-runs that row's `sha` rather than re-resolving the ref.

Folds are **computed per read** until listing measurably hurts; there is deliberately no
denormalized fold column on `builds` yet. Adding one is a cache decision, and a cache that
does not exist cannot go stale or be written to by mistake.

**It is still a reconciler** — the difference is only how the to-be state is arrived at:
`to-be = f(history, timeout)`, then converge (requeue the timed-out, launch the pending).
This unifies two failures that used to need separate machinery — a dead client and a
stalled engine are both just "the event stream stopped advancing past the timeout."

`BuildEvent` carries the `Build` prefix deliberately: "event" is already live in this
domain for GitHub App events and Kubernetes events, and the bare noun would collide.

**`BuildAttempt` is an output type.** It is the srv-side DB model wrapping a finished
result for display; it is not an input to the build path, and the `engine` never sees one.
The pre-rework `AttemptFrom`/`Purpose` vocabulary is discarded.

`BuildResult` is **engine-side only**, and it does not cross this boundary. It survives
there as the engine's result type ([engine.md](engine.md)) because half of it is a live
`*dagger.Container` that could never reach a database anyway; its other half — the scalar
fold — is the very thing the worker persists as `build_events`. So srv reads the fold, not
the struct, and nothing srv-side is typed in terms of it.

Persistence is **display-only** — the engine always re-plans from the current framework
code rather than replaying a stored plan, so a stored event stream can never pin an
outdated build definition. Build **hooks** are deliberately deferred; aborting a run is
itself a hook power, and neither is in this surface yet.

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

This model was **stress-tested against cluster/flux observability** and holds (ADR
[2026-06-29](../decisions/2026-06-29-platform-server-github-app-zero-rbac.md), revised
2026-07-18): a user's view of delivery state is gated by an **infra-repo rights check**,
and the read itself is the **pod ServiceAccount** reading Flux CR state — no platform role
enters. The repo→namespace mapping that observability needs is **routing, not authz** (it
is *not* derivable from the repo name — e.g. `bluepages-infra` →
`haachang.com/s9-haachang`), so it is discovered from existing cluster metadata
(`Kustomization`→`sourceRef`→`OCIRepository`) and cached in the session, never stored as a
permission. The worked derivation is in
[`2026-07-18-srv-rbac-observability.md`](../scratch/2026-07-18-srv-rbac-observability.md).

## Auth mechanism: a GitHub App

`platform` authenticates as a **GitHub App** — the GitHub-sanctioned integration model
(the path GitHub Actions, Vercel, Jenkins, post-migration CircleCI, and Buildkite's
control plane use). Chosen over an OAuth App because it removes the two failure modes an
OAuth-token approach forces you to work around: a stored long-lived per-user secret, and a
bus-factor on whoever connected the repo.

### `srv` owns the App

The server governs one App for its bound org. The App is **created by hand** on GitHub,
guided by the **webui install page** (which renders the running server's live webhook +
callback URLs at install time), then its credentials — **app id, private key, webhook
secret, client secret** — are copied into **fx config**. The old App-Manifest
auto-exchange flow (`/setup/github`) is **killed**: creation is now an install-page step,
credentials arrive via config, and the install record holds only the `installation_id`,
not the credentials. This is a *server install* concern owned by the installer fragment —
**not** `platform init`. See [installation.md](installation.md).

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
local:  Load(".")                      → framework.Units → engine run
CI:     repo-prep: clone url@sha → <wd>      → Load(wd) → framework.Units → engine run
                                               └────────── identical from here ──────────┘
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
