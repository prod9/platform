# Platform Server

The **route surface**, the **install/boot flow**, and the **build lifecycle** are settled:
the [Operations](#operations-settled-surface) table teaches the surface,
[installation.md](installation.md) owns the install model (installer fragment,
`GET /api/install`, boot composition), and [Build lifecycle](#build-lifecycle-event-sourced)
owns the event-sourced record. Held for its own design pass: the **cluster view** — reading
k8s + Flux state for pods, logs, and rollout continuity after a publish.

`srv` is the **second driver** of the one-publish-engine model: the tag-watch server invokes
the same build+push engine the local CLI drives (see
[delivery-verbs-are-orthogonal](../decisions/2026-07-05-delivery-verbs-are-orthogonal.md)
and the one-engine-two-drivers model in [engine.md](engine.md)). The ruling behind its auth
model is
[platform-server-github-app-zero-rbac](../decisions/2026-06-29-platform-server-github-app-zero-rbac.md);
the design it came from is
[platform-as-CI (prior-art)](../scratch/prior-art.md#platform-as-ci-architecture-design-2026-06-29).

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
product fragments — the **auth fragment mounts in both** compositions, because the
org-owner claim needs a login before the server is installed (see
[installation.md](installation.md)) — and aggregates every fragment's
`Migrations` embed into one merged set (`srv/migrate.Merged`, timestamps re-sorted across
fragments) — run by the installer or the CLI, **never at boot**. The fragment import graph
is acyclic — `auth → github`, `builds → {auth, github, install}`, `install → {auth,
github, migrate}` (the org-owner claim is session-gated, so the installer consumes auth;
product fragments may read the bound install settings — that edge carries the settings
read only, never install-flow state; see [installation.md](installation.md)) —
nothing imports `srv` back, and `srv/migrate` is a leaf. `srv/srvtest` holds the
fragment-neutral test scaffolding.

**Install state is stored in fx's settings app** (`fx.prodigy9.co/app/settings`), not a
bespoke table ([installation.md](installation.md), "The install settings"). Its REST
controller ships ungated by design — fx expects the embedder to apply authorization —
so `srv` mounts it through its own wrapper controller: the wrapper's `Mount` builds the
session-auth middleware chain, opens an inner router group, and calls the embedded
`settings.Ctr.Mount` there. The settings migration joins `srv/migrate.Merged` like any
fragment's.

**Data-domain structs stay flat.** There is no ORM here, so a fragment's domain models
mirror the query or fold that produces them — a struct is one row or one reduction, never
a nested object graph (`BuildAttempt` does not carry a `Steps` slice; steps are their own
fold). An API response that spans more than one domain read is a **view**: a wire struct
composed in Go over multiple domain queries/folds, built in the controller layer and
JSON-rendered from there — never a nested shape SELECTed out of the database directly. A
view complicated enough to strain that composition becomes a PostgreSQL view, and the
domain model selects from it like any other relation. This convention is `srv/`'s data
domain only; it says nothing about the shared packages.

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
| `GET /api/repos`            | session                   | repos the App installation reaches, listed **live from GitHub** — never stored        | the webui's repo picker; a stored repo table would be RBAC state the zero-RBAC model forbids                       |
| `GET /api/builds`           | session                   | last 50 builds, newest first                                                          | the webui's build list — the server's whole point made visible                                                     |
| `GET /api/builds/{id}`      | session                   | one build plus its attempts folded — no steps                                         | the build detail view — the stream made readable, which is the reason the events are stored at all                 |
| `GET /api/builds/{id}/steps`| session                   | the build's steps across all attempts, flat, each carrying its attempt ordinal and captured output | steps are a sub-resource: the heavy stdout/stderr payload stays off the detail read                   |
| `POST /api/builds`          | session                   | records a `webui`-triggered build: owner/repo + ref, sha resolved server-side         | the manual trigger — the same domain fact as the webhook, authorized by session instead of HMAC                    |
| `POST /hooks/github`        | App webhook HMAC          | verifies signature; queues a build row per pushed `refs/tags/v*`                      | the pull-model trigger: a version tag *is* the build request (delivery-verbs ADR)                                  |
| `GET /api/install`          | none (installer fragment) | ordered install-state list; served **only while not completely installed**            | drives the SPA installer-vs-app decision ([installation.md](installation.md)); its 404 *is* the "installed" signal |
| `POST /api/install/claim`   | session (installer)       | org-owner claim: resolve installation→org, verify owner, write the `install.*` settings | the first-install gate; the App Setup URL lands on the webui install page, which posts here ([installation.md](installation.md)) |
| `GET/POST/DELETE /api/settings*` | session (installed) / none (installer) | fx's settings app via `srv`'s wrapper controller; the installer composition mounts it ungated — the wizard's credential steps write before login can exist ([installation.md](installation.md)) | operator-visible key/value state — App credentials, install binding, future server settings, one storage |
| `GET /*?go-get=1`           | none                      | vanity go-import meta for module path `platform.prodigy9.co` (the toolchain always appends `go-get=1`) | one host serves module resolution and the product; the standalone `vanity` command and Deployment are legacy |
| `GET /*`                    | none                      | serves the embedded webui at the status the path deserves; the SPA drives installer-vs-app via `GET /api/install`  | single-binary delivery — no separate frontend deploy                                                               |

Session validity and the user's profile are **two operations**, because a webui asks the two
questions at different moments: `GET /api/session` answers "may I still act", `GET
/api/users/me` answers "who am I". The **Flux→srv observability** endpoint `GET
/api/repos/{owner}/{repo}/flux` is **forthcoming** — it belongs to the cluster-view pass and
is not settled here.

### `webui/build/` is committed

The webui (SvelteKit, adapter-static) is embedded via `//go:embed all:build`, which
resolves at compile time — so `webui/build/` — the prerendered file tree of HTML plus
hashed chunks that `pnpm build` emits — is **committed**, rebuilt by hand. Generating it
instead would make `pnpm build` a precondition of `go build`, `go test ./...`, `go run .`,
and the container's `StepTest` alike — a fresh clone would not compile. The Go toolchain
closes no part of that gap; `go build` and `go test` never run `go generate`. Generating
the tree waits on a pre-build hook (the `BeforeBuild` point the
[test-in-build ADR](../decisions/2026-07-05-test-in-build-is-a-hard-gate.md) names,
unbuilt).

### The status of a page is the server's answer, not the browser's

The webui is prerendered to a file tree and embedded, so a fixed route is a file and serving
it is already truthful — nothing matches, nothing exists, 404. A **dynamic** route has no
file: `/builds/123` cannot be enumerated at build time, so it is served from the SPA fallback
page, and a fallback served blindly makes every wrong URL answer 200.

So `srv` decides the status itself and the fallback supplies only the body. A path with a
prerendered file gets that file. A path matching a known dynamic route gets the fallback at
the status the record deserves — 404 when the build does not exist. Anything unrecognized
gets the fallback at 404. The client router then renders the not-found view over a response
that already said so.

The cost is that `srv` knows the webui's dynamic route shapes and looks the record up before
answering — the price of a static UI, and the reason a status is never left to the browser to
infer. A wrong URL that answers 200 is a lie told to every crawler, monitor, and `curl` that
ever reads it.

**The server always boots.** A DB it cannot reach is an install-state error rather than a
boot failure, and **migrations never run at boot** — they are the installer's button or
`./platform srv data migrate` ([installation.md](installation.md)). Boot's one decision is
the API composition, taken once from `install.GetState()`: the installer fragment while the
server is incomplete, the product fragments once it is installed. Nothing at boot touches the
build queue; executing builds is the worker's, and the queue is the records themselves.

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
was asked for*. The webhook gets the sha from the push payload; the manual trigger
(`POST /api/builds`) names only a ref, so its controller resolves it to a sha via the
GitHub API before recording — resolution is part of validating the request, not of
executing it.

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
scaled by adding processes. A process runs **one job at a time** and draws from the single
queue, so a long build occupies a whole process and no process can be pointed at a job kind
([fx-worker.md](../vendor/fx-worker.md)); one worker is enough to make progress — the scan
runs in the gap after each build — but builds serialize and queue latency tracks build
duration. Parallelism across job kinds waits on a partitioning capability fx does not have. The job *code* lives in the fragment that owns its domain, per
fx's self-contained-fragment convention: the build jobs are files in `srv/builds`, a session
sweep would belong to `srv/auth`. There is no central jobs package — that is the grab-bag
fx's fragment model exists to prevent. A build loop hand-rolled *inside* an HTTP fragment is
the shape this forbids: the server process serves, and the worker process works.

**The worker is general background processing**, not a build runner: recurring cleanup
sweeps, reconciliation of bad state, and anything else that must happen off the request path
are jobs too. fx's queue is one-shot, so a recurring job reschedules itself at the end of
`Run` — no cron machinery is added.

**Two jobs carry a build**, and the split is what keeps the record the queue:

| Job           | Shape                        | What it does                                                                                                                                                                                |
| ------------- | ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `scan_builds` | recurring, singleton         | Reads `builds` against their events, finds every build nothing has reported on, and schedules a `build` job per build with `ScheduleNow`.                                                   |
| `build`       | one-shot, payload = build id | Repo-prep → engine run → write `build_events`.                                                                                                                                              |

A controller therefore never schedules a job. It appends a record; the scan turns records
into work. That is what makes the trigger sources interchangeable.

**A job's name is fx's dispatch key, and its struct is its payload.** `worker` registers one
instance per `Name()` and unmarshals each queued row's payload into that instance before
`Run`, so many pending `build` jobs coexist under the one name and are told apart only by the
build id they carry. `ScheduleNowIfNotExists` dedupes on the name alone, which makes it the
primitive for a **singleton** job and never for a per-build one: `scan_builds` schedules
*itself* with it at the end of every run, and schedules each build with plain `ScheduleNow`.

**A build with an event has been picked up, and the scan leaves it alone.** The first thing
the `build` job does is report, so the presence of any event is what tells an overlapping
sweep that this build already has a worker — which is what keeps one build from being run
twice.

**Recovering a stalled build is not yet in this surface.** The obvious rule — reschedule a
build whose last event is older than its timeout — needs two things this design does not have
yet: a stall boundary the scan can know (a unit's timeout lives in the repo's
`platform.toml`, which only exists once the tree is prepared, so the scan cannot read it),
and an attempt boundary a resumed stream cannot blur (a span closes only when every reporter
has finished, so appending to a stalled attempt extends it rather than starting a new one).
Until both are settled, a stalled build stays stalled and a human retries it — which appends
a new row and is the path that already works.

**The publish tag is the ref's last segment.** A build's `ref` is `refs/tags/vX.Y.Z` and the
image is published under `vX.Y.Z` — the worker strips the `refs/tags/` prefix and passes the
remainder as the tag, so the image carries the version a human pushed rather than a sha.

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

**A retry is just a new build.** Clicking retry records the same domain fact the manual
trigger records — `POST /api/builds` with the build's repo and ref, ref re-resolved — and
the superseded build simply runs out; there is no cancel machinery (aborting is a hook
power, deferred). No dedicated retry endpoint exists. The schema's `trigger = 'retry'` and
`retry_of` stay for a linked retry chain, written by nothing in this surface.

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
guided by the **webui install wizard** (which renders the running server's live webhook +
callback URLs at install time), then its credentials — **app id, private key, webhook
secret, client secret** — are pasted into the wizard's credential steps and saved as the
**`github.app_*` settings**. Creation is a wizard step rather than an App-Manifest
auto-exchange: credentials live in the settings table (`srv/github`'s `LoadApp` reads
them; srv and the worker share the rows), and the deployment's fx config carries only
`DATABASE_URL` and the listen address. This is a *server install* concern owned by the
installer fragment — **not** `platform init`. See [installation.md](installation.md).

### Two token types, chosen per operation

| Token                  | Identity            | Scope                                        | Used for                                        |
| ---------------------- | ------------------- | -------------------------------------------- | ----------------------------------------------- |
| **Installation token** | `platform[bot]`     | installed repos ∩ granted permissions, ~1h   | webhook-driven / autonomous work (clone, build, publish) |
| **User-to-server**     | the triggering user | (user's access) ∩ (app's granted perms)      | where attribution + per-user gating matter (a deploy) |

- **Installation token** — minted from the app key (JWT → installation), app/bot identity,
  short-lived. No bus-factor; commits attributed to `platform[bot]`.
- **`srv/github` owns the App API client**: the App JWT, installation-token minting, and
  the App-identity queries the server makes (installation→org resolution, org-owner
  check, the installation's repo list, ref→sha resolution). Fragments consume it; none
  talks to GitHub's API directly except auth's own user-OAuth exchange.
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

**The clone authenticates with the installation token** (§Two token types): repo-prep
mints one per sync and injects it into the fetch URL for that command only — the token is
~1h-lived and autonomous work is exactly what the installation identity is for. Nothing
long-lived lands on disk; the mirror's stored remote stays credential-free.

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

Each layer consumes the one below *after* it works. The CLI delivery path, the `srv`
wrap (webhook ingest, auth, the build pipeline), the **App API client** (`srv/github`:
JWT, installation token, the App-identity queries), the **org-owner claim** (the
first path to a completely-installed server), the **credentialed clone** (repo-prep
authenticating with a per-sync installation token), the **manual trigger + repo
list** (`POST /api/builds`, `GET /api/repos`), and **build detail + truthful
statuses** (`GET /api/builds/{id}`, the `/steps` sub-resource, the SPA fallback
served at the status the record deserves), and the **webui** on top of the proven
API have shipped; what remains:

1. **Cluster install** — declared in the `prod9/infra` GitOps repo. The srv pod runs as
   its own **ServiceAccount** with read-only RBAC on the Flux CRs (the forthcoming
   cluster-view surface reads them); platform deploys nothing — publish pushes the image
   and Flux pulls.

## Open details (not blockers)

- Where the `init` server marker lives — `platform.toml` `[server]` field vs CLI-global
  config.
- User-token expiry policy — current default: the user token is stored as received
  with no refresh handling (pair it with the App's non-expiring setting); the platform
  session lasts 30 days. Expiring tokens + refresh return here if the balance shifts.
