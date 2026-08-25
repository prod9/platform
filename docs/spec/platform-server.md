# Platform Server

The **route surface**, the **install/boot flow**, and the **build lifecycle** are settled:
the [Operations](#operations-settled-surface) table teaches the surface,
[installation.md](installation.md) owns the install model (installer fragment,
`GET /api/installation`, boot composition), and [Build lifecycle](#build-lifecycle-event-sourced)
owns the event-sourced record. Held for its own design pass: the **cluster view** — reading
k8s + Flux state for pods, logs, and rollout continuity after a publish.

The `srv` + worker pair is the **CI/CD server driver**, peer to the local CLI driver.
`srv` records build intent and the worker asynchronously invokes the same build/publish
capabilities the local CLI drives (see [`execution-modes.md`](execution-modes.md), the
[execution-mode decision], and the one-engine-two-drivers model in [engine.md](engine.md)).
The ruling behind its auth model is
[platform-server-github-app-zero-rbac](../decisions/2026-06-29-platform-server-github-app-zero-rbac.md);
the design it came from is
[platform-as-CI (prior-art)](../scratch/prior-art.md#platform-as-ci-architecture-design-2026-06-29).

[execution-mode decision]: ../decisions/2026-08-24-execution-mode-does-not-define-delivery-policy.md

## What `srv` is

🚨 **`srv` is a web app, and it carries a web app's own conventions.** It is built on
[prod9/fx](https://fx.prodigy9.co) and its shape — fragments, controllers, actions,
background jobs, embedded migrations, how finely it splits packages — is decided by **fx
web-app convention**, not by the laws written for the CLI and the shared packages. The
package-layout rules in [architecture.md](architecture.md) govern the shared-package graph
and hold **no jurisdiction** here; citing them to settle a question about `srv`'s internals
is a category error. A reader who wants to know how a piece of `srv` should be shaped reads
fx, then this file.

`srv` is the API + webhook processor: every push for a registered repository records a
build of the exact commit; the worker later builds it and applies the manifest's server
publish policy. It owns the GitHub
App, the DB, and token minting. It is a layer above the **shared packages** (the stateless
build/render/publish machinery: `framework`, `engine`, `gitops`, …)
and consumes them per request. The remote-build facade owns a short-lived engine `Session`
for each module operation because srv retains no live container afterward. Local callers
that need a result's container continue to hold their session explicitly; see
[engine.md](engine.md), §`Session` — the unit of lifetime.

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

Internally `srv` is an **fx application composed on fx's sanctioned public surface** — one
self-contained fragment subpackage per concern (`srv/auth`, `srv/github`, `srv/builds`,
`srv/repos`, `srv/install`, `srv/system`), each declaring an fx app fragment
(`app.Build()`) that carries its controllers and, where it owns tables, its
`EmbedMigrations` (`github` is config-only, no schema; the fx settings app's schema
arrives by mounting `settings.App`, never by reaching into its migrations accessor).
Platform retains its own Cobra root and composes fx manually through the public
collectors: `RegisterMigrations` registers the complete fragment tree, while
`CollectCommands`, `CollectJobs`, and `CollectFragment` supply its commands, worker jobs,
and HTTP application. The data command is built with those registered migration sources;
srv never reaches into fx internals or recreates an fx composition walk.

Every fragment — installer and product alike — is composed permanently. A request-time
gate reads `install.IsInstalled` (the claimed record, read install-safe —
[installation.md](installation.md) §Boot composition): before claim it exposes
the installer UI/API and auth while gating product API requests; after claim it exposes
the product and returns 404 from installer UI/API routes. The claim changes the reachable
surface without restarting the process. fx's lazy `AddDataContext` middleware remains on
the full HTTP stack, so DB-less boot uses the framework's ordinary per-request connection
failure rather than a second composition. The webui fallback-status handler remains the
other platform-specific seam. Where fx lacks an affordance, the fix routes to fx, never a
local workaround.

**Migration sources are fx's concern, not srv's.** Each fragment's SQL reaches fx's
registry through its own `EmbedMigrations` declaration, walked by fx's app composition
— srv never calls `migrator.Embed`, threads no `migrator.Source` values, and holds no
merge code. Every migration consumer reads fx's default source (`migrator.FromAuto`) —
the same composition fx's own CLI and middleware use. The only migration knowledge in
srv lives in two deliberately separate operations: the install fragment owns its
pre-install migration step, while the system fragment owns post-install schema
observation and remediation. They may duplicate the small amount of fx `Plan`/`Apply`
plumbing rather than importing one another. The CLI remains a third entry point, and
none runs **at boot**. The jobs table is fx worker's own concern
(`worker.Start` creates it), not a srv migration.

Every migration carried by a published platform release is treated as applied to a live
installation and is immutable: its filename, sequence, up SQL, and down SQL never change.
Later schema or data work is always a new forward migration. Editing a published migration
requires chakrit's explicit approval for that specific edit; refactors and general migration
work do not grant it. `resync-migrations` is an operator recovery mechanism, never an
authoring workflow or permission to rewrite migration history.

The two HTTP operations share fx's migration mechanism, not one domain action. The
installer operation is **bootstrap**: against an empty or partially built database it
applies the complete pending set, and success must leave the fx settings migration
applied so the following wizard steps have somewhere durable to store credentials. Its
response is the refreshed installer checklist. The system operation is **steady-state
upgrade remediation**: against an already claimed installation it reports and applies
newly shipped pending migrations, and responds only with the fresh operational migration
plan. The response preserves fx's ordered, line-by-line plan: one item per
`migrator.Plan`, projected as `{action,migration}` without exposing
the migration's SQL. The server assigns no presentation category to an item: action is
the fact, and the client decides how each action renders. A dirty plan still makes the
HTTP run action refuse rather than applying resync or prune remotely; recovery is the
operator deliberately running `./platform srv data resync-migrations --force` in a
server shell. Platform neither folds the plan into aggregate counts nor performs resync
over HTTP. An empty plan means the schema is current; a successful POST returns the newly
empty plan.

**`srv/system` is the post-install operational surface** — how an installed server is
observed and kept current from inside the product composition: the masked install-facts
read (`GET /api/system/settings`) and the schema state and remediation
(`GET`/`POST /api/system/migrations`). Its migration operation is post-install product behavior;
the installer's pre-install migration step remains owned and implemented by
`srv/install`. Neither fragment delegates migration execution to the other.

The fragment import graph
is acyclic — `auth → github`, `builds → {auth, github, install}`, `repos → {auth,
github, install}`, `install → {auth, github}`, `system → {auth, github, install}`
(the org-owner claim is session-gated, so the installer consumes auth;
product fragments may read the bound install settings — that edge carries the settings
read only, never install-flow state; see [installation.md](installation.md)) —
nothing imports `srv` back. `srv/srvtest` holds the shared test scaffolding
(database setup off fx's default source, the App stub); it imports `github` for the
stub, which is why `github`'s own tests cannot use it.

**Install state is stored in fx's settings app** (`fx.prodigy9.co/app/settings`), not a
bespoke table ([installation.md](installation.md), "The install settings"). The settings
app contributes its schema only — srv mounts no settings REST surface. Every write goes
through a purpose-built installation resource (`POST /api/installation/server`,
`POST /api/installation/organization`, `POST /api/installation/github-app`,
`POST /api/installation/credentials`, `POST /api/installation/registry`, the claim) or
the model accessors (`settings.Get`/`Upsert`) directly; a generic key/value API would be
an unauthenticated-write surface pre-install. Post-install the one reader is
`GET /api/system/settings` — session-gated, read-only, and masking: a secret-valued key
(private key, client secret, webhook secret, registry token) serves a masked
placeholder, never the value, so the settings page can show *that* a credential is
present without the server ever replaying it. The settings migration reaches the
merged set the same way as any fragment's — registered with `migrator.Embed`.

**Data-domain structs stay flat.** There is no ORM here, so a fragment's domain models
mirror the query or fold that produces them — a struct is one row or one reduction, never
a nested object graph (a module result does not carry a `Steps` slice; steps are their own
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
| `GET /auth/github`          | none                      | binds the return + pre-claim installation, then redirects to GitHub user OAuth         | login entry point — platform delegates identity to GitHub, holds no passwords (identity ADR)                       |
| `GET /auth/github/callback` | state cookie              | exchanges the code, `GET /user`, find-or-create user+identity, mints a session cookie | completes login; identity keyed on immutable provider id so GitHub renames don't break links (identity ADR)        |
| `GET /api/session`          | session                   | session state — expiry + user id; 401 when none                                       | the webui's "is my session valid" probe, distinct from the user's profile                                          |
| `DELETE /api/session`       | none (cookie optional)    | deletes the session row, clears the cookie                                            | session revocation server-side — a stolen cookie dies with the row, not with the browser                           |
| `GET /api/users/me`         | session                   | the session user's profile (id + name)                                                | the webui's "who am I" — profile, not session validity                                                             |
| `GET /api/repos`            | session + read filter     | the **registered** repos, filtered against the session user's GitHub authorization     | the repos landing page; registration is stored, permission never is (see §Repos are registered, authorization is GitHub-derived) |
| `GET /api/repos/candidates` | session + read filter     | repos both the user and App reach that are **not yet registered**                      | the onboarding wizard's pick list, minus what is already onboarded                                                |
| `POST /api/repos`           | session + repo write      | stores the reviewed repo, raw manifest, resolved publish policy, and modules           | onboarding records exactly what the user reviewed, not whatever the default branch points at later; unreachable repo is 404, absent manifest is 409 |
| `GET /api/repos/{owner}/{repo}/manifest` | session + repo read | returns the default-branch sha, parsed manifest, and resolved publish policy       | the onboarding review supplies the immutable sha that confirmation sends back; unreachable repo is 404, absent manifest is 409 |
| `GET /api/repos/{owner}/{repo}/builds` | session + repo read | the repo's builds, newest first; `?limit=N` caps the page                          | builds nest under a repo in the UI; the landing page fans out `?limit=3` per visible repo                          |
| `GET /api/builds`           | session + read filter     | last 50 authorized builds, newest first                                               | the global feed cannot disclose builds from inaccessible repos                                                     |
| `GET /api/builds/{id}`      | session + repo read       | one build plus its selected modules and their folded states — no steps                 | the build detail view — the streams made readable, which is the reason the events are stored at all                |
| `GET /api/builds/{id}/steps`| session + repo read       | the build's steps across all selected modules, flat, each carrying its build-module id and captured output | steps are a sub-resource: the heavy stdout/stderr payload stays off the detail read                 |
| `POST /api/builds`          | session + repo write      | records a `webui`-triggered build: owner/repo + ref, sha resolved server-side; may carry a module list | the manual trigger — the same domain fact as the webhook, authorized by the user's write access; module selection is the manual trigger's alone (§Triggering a build) |
| `GET /api/engines`          | session + read filter     | the engine fleet with repo-attributed work filtered by GitHub authorization            | the engines page — the fleet the builds run on, read from the same `DAGGER_ENGINE` source the worker dials ([engine.md](engine.md) §Runner discovery) |
| `GET /api/engines/{addr}`   | session + read filter     | one engine instance; repo-attributed work is filtered by GitHub authorization          | the engine detail page; the instance is named by its resolved `host:port`, URL-encoded                           |
| `GET /api/system/settings`         | session                   | the install-time facts, read-only; secret-valued keys are served **masked, never the value** | the System / Settings page — the one post-install reader of the install settings (`srv/system`)                             |
| `GET /api/system/migrations`       | session                   | the ordered migration plan, one projected fx plan item per line; empty means current | the System / Migrations page; the client interprets each action for presentation |
| `POST /api/system/migrations`      | session                   | applies a clean pending migrate plan; response is the freshly planned result           | the post-install run button owned by `srv/system`; distinct from the installer's pre-install migration operation |
| `POST /hooks/github`        | App webhook HMAC          | records a registered repo's non-deleted push after signature verification              | registration admits builds; a valid push to an unregistered repo is ignored                                        |
| `GET /api/installation`          | none (installer fragment) | ordered installation-state list; served **only while the server is unclaimed**       | drives the SPA installer-vs-app decision ([installation.md](installation.md)); its 404 *is* the "installed" signal |
| `POST /api/installation/claim`   | session (installer)       | creates the org-owner claim: resolve installation→org, verify owner, write the `install.*` settings | the first-install gate; the App Setup URL lands on the webui installation page, which posts here ([installation.md](installation.md)) |
| `POST /api/installation/github-app` | none (installer)      | updates the GitHub App resource with app id, app slug, client id, and webhook secret | what GitHub's creation form yields, saved as its own wizard step ([installation.md](installation.md)) |
| `POST /api/installation/credentials` | none (installer)     | updates the credentials resource with the private key and client secret              | the keys GitHub generates after creation; both App steps write before login can exist — same ungated posture as the migrations button ([installation.md](installation.md)) |
| `POST /api/installation/registry` | none (installer)      | updates the registry resource with the ghcr push PAT                                  | ghcr accepts no App-derived credential ([vendor/ghcr-auth.md](../vendor/ghcr-auth.md)); same ungated posture |
| `POST /api/installation/server`  | none (installer)       | updates the server resource with its public URL                                       | the one server-side truth of where the deployment lives — OAuth redirects derive from it ([installation.md](installation.md)) |
| `POST /api/installation/organization` | none (installer) | updates the organization resource with its primary-org slug                           | the slug every wizard GitHub link is built from ([installation.md](installation.md)); same ungated posture |
| `POST /api/installation/migrations` | none (installer)    | creates the pending migration applications                                            | the wizard's run-migrations button; re-runnable, applies only what is missing ([installation.md](installation.md)) |
| `GET /*`                    | none                      | serves the embedded webui at the status the path deserves; the SPA drives installer-vs-app via `GET /api/installation` | single-binary delivery — no separate frontend deploy |

**Module resolution is a static fact, not a route.** `go get platform.prodigy9.co`
resolves through a `go-import` meta tag baked into the SPA's page shell — every
served page carries it, including the 404 fallback (the toolchain parses meta out
of non-200 bodies and re-fetches the declared prefix;
[vendor/go-vanity-imports.md](../vendor/go-vanity-imports.md)). The tag names the
module path and repo literally; no server code, no configuration, and no
`?go-get=1` handling exist. The standalone `vanity` command and Deployment are
legacy.

Session validity and the user's profile are **two operations**, because a webui asks the
two questions at different moments: `GET /api/session` answers "may I still act", `GET
/api/users/me` answers "who am I". A login session has an absolute **two-hour lifetime**;
activity never extends it. On expiry, every session-gated API operation returns `401` and
the webui reauthenticates through the flow below rather than presenting the refusal as an
operation error.

`GET /auth/github` accepts a same-origin relative return location. The OAuth state binds
that location to the login attempt, so the callback can trust it without admitting an open
redirect. A successful callback replaces the stored GitHub user token, mints a fresh
two-hour session, snapshots the repositories and permission levels that both the user and
App can reach, and redirects to the bound location. The location includes path and query;
the webui preserves the browser-only fragment across the round trip. Missing or invalid
return locations fall back to `/`.

Before the server is claimed, the install webui also sends the GitHub App
`installation_id` already present in its setup URL. OAuth state binds that id beside the
return location, and the callback uses it to build the first authorization snapshot before
`install.Bound` exists. After claim, the callback takes the installation id from the bound
installation record; a request parameter cannot select another installation.

OAuth denial, callback failure, or missing callback inputs redirects to the webui's
`/session/` recovery state with the bound return location. The page states that sign-in did
not complete and offers one explicit retry; it never redirects automatically. The retry
starts a new OAuth attempt, while a successful callback returns directly to the interrupted
location.

The **Flux→srv observability** endpoint `GET
/api/repos/{owner}/{repo}/flux` is **forthcoming** — it belongs to the cluster-view pass and
is not settled here.

### GitHub is the repository authorization boundary

Platform stores no durable repo permissions. Login asks GitHub once for the repositories
and permission levels reachable by both the user and the App installation, then records
that authorization snapshot against the new session. Every user-initiated repo operation
reads the snapshot: reads require GitHub read access and mutations require GitHub write
access. An App-only installation token is never sufficient for a user-initiated operation.

| Activity                                                                 | User gate      | App gate             |
|--------------------------------------------------------------------------|----------------|----------------------|
| List registered or candidate repos                                       | read           | repo access          |
| Read manifest, repo builds, build detail, steps, logs, or delivery state | read           | repo access          |
| Read a global feed or repo-attributed engine view                         | filter by read | filter by access     |
| Register a repo, trigger a build, or retry a build                        | write          | repo access          |
| Receive a push webhook                                                    | none           | HMAC + registration  |
| Clone, build, publish, or record autonomous worker results                | none           | operation permission |

An id-addressed read first resolves its repository, then applies the same gate; missing and
inaccessible resources are both `404`, so build ids and dynamic page fallback statuses do
not disclose repo existence. A global or fleet read returns only rows attributed to repos
passing the read gate.

The snapshot is a session-owned cache keyed by session and GitHub repository id, carrying
the repository identity and read/write level. It is deleted with the session and is never
reused by another session. This makes ordinary platform reads local and bounds stale GitHub
permission to two hours without a permission-sync table or a GitHub request per operation.
The migration introducing snapshots revokes sessions minted under the earlier model: they
have no trustworthy cached authorization to preserve, and one fresh login rebuilds it.

The session lifetime remains the revocation fallback even if event-driven invalidation is
added later. A future tightening may consume GitHub organization-membership,
team-membership, collaborator, App-authorization, and installation-repository changes to
delete affected users' sessions; those events accelerate revocation but never become an
authorization database.

### Repos are registered, authorization is GitHub-derived

The `repos` table records **registration** — *this repo is onboarded to build here* — and
nothing else. It is a product fact, not a permission: no role, no access bit, no cached
GitHub state lives on it. `GET /api/repos` intersects the registered set with the current
session's GitHub-derived authorization snapshot. Losing GitHub permission removes platform
access no later than the end of that two-hour session, registration row or not.

```
repos                           -- registration only: this repo is onboarded to build here
  id            bigserial
  owner         text
  repo          text            -- UNIQUE (owner, repo)
  registered_by bigint          -- REFERENCES users(id)
  created_at    timestamptz
```

Every manifest observation is immutable and commit-addressed. Raw text preserves exactly
what the repository contained; the parsed records preserve what this platform version
understood from it. Those are different historical facts, written together, never a mutable
cache. The current repository manifest is the newest observation by `created_at`; `repos`
holds no current-manifest pointer to synchronize.

```
repo_manifests                  -- one immutable platform.toml observation
  id            bigserial
  repo_id       bigint          -- REFERENCES repos(id)
  sha           text            -- exact commit read
  raw           text            -- exact platform.toml bytes
  maintainer    text
  repository    text
  platform      text            -- deprecated input, preserved after parsing
  local_arch    text
  publish_arch  text
  strategy      text
  server_publish text          -- resolved 'always' | 'tags' | 'never'
  excludes      text[]
  vars          jsonb
  created_at    timestamptz
                                -- UNIQUE (repo_id, sha), UNIQUE (id, repo_id, sha)

repo_manifest_modules           -- complete parsed modules for one observation
  id            bigserial
  manifest_id   bigint          -- REFERENCES repo_manifests(id)
  name          text
  workdir       text
  timeout_ns    bigint
  framework     text
  env           jsonb
  port          integer
  command_name  text
  command_args  text[]
  asset_dirs    text[]
  build_dir     text
  go_version    text
  image_name    text
  package_name  text
                                -- UNIQUE (manifest_id, name), UNIQUE (id, manifest_id)
```

The arrays above are ordered value fields (`command_args`, `asset_dirs`, `excludes`), and
the JSON objects are open maps whose keys are part of `platform.toml` (`env`, `vars`); none
is a relation disguised as a collection. A later build references these rows rather than
serializing `framework.BuildUnit`, which is runtime behavior after interpretation.

Onboarding is a wizard ([webui.md](webui.md)): pick from `GET /api/repos/candidates` (the
App-reachable repos not yet registered, live), then review `GET
/api/repos/{owner}/{repo}/manifest`. That read resolves the default-branch head and returns
its commit sha, parsed modules, and resolved `[server].publish` policy. Confirmation posts
owner, repo, and that sha. The server re-reads `platform.toml` at the supplied sha,
parses it again, resolves the same policy, and transactionally inserts the repo, raw
snapshot, parsed policy, and every parsed module. The browser never sends manifest content
back across the
trust boundary, and a moving default branch cannot change what gets registered after
review. Registration is the only write; deregistration is not in this surface yet.

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
boot failure, and **migrations never run at boot** — they are the installer's button, the
System / Migrations page (`POST /api/system/migrations`), or
`./platform srv data migrate` ([installation.md](installation.md)). The full fx application
is composed regardless of install state; `install.IsInstalled` controls the request-time
surface, not boot. Nothing at boot touches the build queue; executing builds is the
worker's, and the queue is the records themselves.

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

**The records are the queue.** A build is the aggregate request; its `build_modules` are the
independently executable work. Pending is derived from those domain records and their event
streams, never stored as a status and never delegated to fx's mechanism-level `jobs` table.
This is what makes trigger sources interchangeable: each materializes the same facts.

**The record must be complete enough to act on.** The controller resolves the ref, reads
and parses `platform.toml` at that exact sha, and atomically writes the build, its immutable
manifest snapshot, and its selected `build_modules`. The webhook gets the sha from the push
payload; the manual trigger (`POST /api/builds`) names a ref, so its controller resolves it
through GitHub first. Neither dispatcher nor executor re-derives what was asked for.

**A webhook build is whole-repo; a manual trigger may select modules.** `platform.toml`'s
`[modules]` defines which modules exist. A webhook materializes one `build_modules` row for
every module in the snapshot; a manual trigger materializes only the requested subset. An
empty selection therefore means no work and is rejected — there is no empty-means-all
convention. A selected name absent from the snapshot is rejected before the build exists.
The srv database and API call these records **modules**; **unit** begins only when
`framework.Units` interprets one into a runtime `framework.BuildUnit`.

**Build cadence and publish cadence are separate.** Every non-deleted push to a registered
repository records a build, whether its ref names a branch or any tag. After a successful
build, the immutable manifest observation's resolved `server_publish` value selects the
policy: `always` publishes under `latest`, `tags` publishes tag builds under the exact
tag, and `never` does not publish. No `v` prefix has server meaning. The absent-field
default is
the repository-name rule in [`execution-modes.md`](execution-modes.md), never release
strategy or framework inference.
[`execution-modes.md`](execution-modes.md) owns the complete boundary.

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

**Two jobs carry a build**, and the split makes a module the unit of capacity and failure:

| Job               | Shape                               | What it does                                                                                                                      |
|-------------------|-------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------|
| `dispatch-builds` | recurring, singleton                | Finds unclaimed build modules and schedules one `build-module` job for each; repeated scans reconcile missed scheduling.          |
| `build-module`    | one-shot, payload = build-module id | Claims one module, constructs the engine request and persistence Observer, calls engine once, and records its report.             |

All fx job names are dash-separated slugs.

A controller therefore never schedules a job. It appends a complete build aggregate; the
dispatcher turns its module records into fx jobs. `ScheduleNow` writes mechanism state and
the domain rows remain authoritative, so the dispatcher is deliberately idempotent and
reconciles any failure between those writes.

**A job's name is fx's dispatch key, and its struct is its payload.** `worker` registers one
instance per `Name()` and unmarshals each queued row's payload into that instance before
`Run`, so many pending `build-module` jobs coexist under one name and are told apart by
their build-module ids. The dispatcher is the singleton job; module jobs are ordinary
one-shot jobs.

**A module claim makes duplicate delivery harmless.** Scheduling through fx and recording
domain intent are separate writes, so the dispatcher is at-least-once. A `build-module` job
reads `os.Hostname()` and atomically claims the module:

```sql
UPDATE build_modules
SET claimed_at = now(),
    claimed_by = $2
WHERE id = $1
  AND claimed_at IS NULL
RETURNING *;
```

The one caller that receives a `BuildModule` proceeds; every duplicate receives no row and
exits without executing. Hostname lookup fails before the claim rather than recording an
unattributed worker. Once claimed, a module is never automatically rescheduled. A worker
dying afterward leaves visible stalled work for an operator, whose retry creates a new
build aggregate rather than mutating this one.

**The server chooses the publish tag from the manifest's server policy.** Under `tags`,
the worker strips `refs/tags/` and publishes under the entire remaining tag name; tags
need no `v` prefix, and branch builds do not publish. `always` publishes under `latest`
regardless of the triggering ref. `never` builds without publishing. This policy
belongs to the server driver, not the engine and not the local `./platform publish`
command.

**The publish credential is the wizard-saved registry token.** The worker derives the
registry host from the selected persisted manifest module's image name, reads
`registry.<host>.token`, and feeds the engine's `REGISTRY`/`REGISTRY_USERNAME`/
`REGISTRY_PASSWORD` config — username = the installation record's `installed_by_login`
([installation.md](installation.md), "The registry token";
[vendor/ghcr-auth.md](../vendor/ghcr-auth.md)). A missing token fails that module's run
outright — the server never attempts an unauthenticated push. Each module job derives its
own registry host, so independently built modules may publish to different registries.

🚨 **A job's success is not a build's success.** A job answers *did the job do its work* —
relay the instruction to the engine, observe the execution, record what happened. A build
answers *did the build succeed*, and that answer lives only in `build_events`. A build that
failed and was correctly recorded is a **successful job**. So `Run` returns an error only
when the job itself could not do its work; a failed build returns nil. Collapsing the two
vocabularies would put build state back in fx's `jobs` table, which is the mechanism's, not
the domain's.

**A build job is not called a "runner."** The fx worker executes jobs; the engine facade
owns builds; Dagger runners are the execution endpoints engine selects. Three live
concepts, three distinct words.

## Build lifecycle: event-sourced

There is **no stored build `status`.** Execution history is an append-only **`BuildEvent`**
stream in a `build_events` table. It covers the whole module lifecycle by transcribing
what engine reports through its `Observer` ([engine.md](engine.md)); engine never
serializes. The database *is* the channel; the webui reads it back. Nothing subscribes to
a live in-process stream across the process boundary, which is exactly why engine needs
no late-joining observer.

The event order is:

```
clone_started -> clone_done -> run_started -> step_started / step_done
              -> image_built -> published -> run_done
```

`clone_done`, `step_done`, and `run_done` carry the error for the span they close. A clone
failure ends at `clone_done` and never invents an engine run. Engine emits `run_started`
after repository materialization and before config loading and unit interpretation. Before
the first `step_started`, `EngineAssigned` records the selected endpoint and assignment
time on `build_modules`. Every run phase ends with `run_done`; `published` is absent from
a build-only run.

Display state is a **fold** of each module row and its event stream:

| Fold              | Computed as                                                      |
|-------------------|------------------------------------------------------------------|
| module state      | claim + engine-assignment fields and one module's events         |
| build state       | reduction of all selected module states                           |
| stuck / timed-out | latest module transition vs the persisted module timeout          |

There is no attempt model. A build module executes once; a failed or stalled execution
remains history, and operator retry creates a new build with new module rows.

### Build tables

```
builds                          -- one aggregate request per trigger; immutable after insert
  id            bigserial
  trigger       text            -- 'github-push' | 'webui' | 'cli' | 'retry'
  retry_of      bigint NULL     -- REFERENCES builds(id); set only when trigger = 'retry'
  user_id       bigint          -- REFERENCES users(id); the system user for a webhook trigger
  repo_id       bigint          -- REFERENCES repos(id)
  manifest_id   bigint          -- REFERENCES repo_manifests(id), same repo + sha
  clone_url     text
  ref           text            -- 'refs/heads/main' | 'refs/tags/v1.2.3'
  sha           text            -- the commit this build builds
  created_at    timestamptz
                                -- composite FK (manifest_id, repo_id, sha)
                                --   REFERENCES repo_manifests(id, repo_id, sha)

build_modules                   -- the explicit selected subset; one row = one worker job
  id            bigserial
  build_id      bigint          -- REFERENCES builds(id)
  manifest_id   bigint
  manifest_module_id bigint      -- REFERENCES repo_manifest_modules(id)
  claimed_at    timestamptz NULL
  claimed_by    text NOT NULL DEFAULT ''  -- claiming worker's os.Hostname()
  engine_host   text NOT NULL DEFAULT ''  -- selected host:port
  engine_assigned_at timestamptz NULL
  created_at    timestamptz
                                -- UNIQUE (build_id, manifest_module_id)
                                -- composite FKs require build + module to share manifest_id
                                -- CHECK ((claimed_at IS NULL) = (claimed_by = ''))
                                -- CHECK ((engine_assigned_at IS NULL) = (engine_host = ''))

build_events                    -- append-only; one row per module lifecycle event
  id            bigserial
  build_module_id bigint        -- REFERENCES build_modules(id)
  kind          text            -- clone_started | clone_done | run_started
                                -- step_started | step_done | image_built | published | run_done
  step          text            -- '' unless step-scoped
  at            timestamptz     -- event time, not the insert time
  error         text            -- clone_done, step_done, run_done
  image         text            -- image_built, published
  hash          text            -- published only
  stdout        text            -- captured output, per step
  stderr        text
  created_at    timestamptz
```

The composite foreign keys make two mismatches unrepresentable: a build cannot name a
manifest from another repository or commit, and a `build_modules` row cannot select a
module from another manifest.

`build_events` transcribes the engine's reporting callbacks ([engine.md](engine.md)). Its
module identity is the
`build_module_id` foreign key, not a copied unit name. `at` preserves the time at which
engine observed the event, so elapsed time survives a slow writer. Engine
assignment is module metadata rather than a repeated stream fact: `EngineAssigned` updates
`build_modules.engine_host` and `engine_assigned_at`, which engine detail reads directly.
Captured `stdout`/`stderr` ride the `step_done` row rather than a kind of their own.

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

**Retry is operator-only and creates a new build.** A failed module is terminal history;
the dispatcher never retries it automatically. Clicking retry records the same domain fact
the manual trigger records — `POST /api/builds` with the build's repo and ref, re-resolved
and re-snapshotted — while `retry_of` links the new aggregate to its predecessor. There is
no cancellation machinery.

**Modules are independent, including in a monorepo.** The build model carries no module
dependency graph and the dispatcher imposes no order. A developer encodes any required
cross-module preparation inside each module's own build definition, so every module remains
independently buildable. Queue fairness across large and small builds is deliberately
deferred until observed scale makes it a real requirement.

Folds are **computed per read** until listing measurably hurts; there is deliberately no
denormalized fold column on `builds` yet. Adding one is a cache decision, and a cache that
does not exist cannot go stale or be written to by mistake.

**Dispatch is still reconciliation.** Its to-be state is one fx job for every module whose
`claimed_at` is null; duplicate delivery loses the guarded update and does no work. Failed
and stalled module executions are terminal until an operator creates a new build.

`BuildEvent` carries the `Build` prefix deliberately: "event" is already live in this
domain for GitHub App events and Kubernetes events, and the bare noun would collide.

**A module result is an output fold.** It is the srv-side display model reduced from one
build module's events; it is not an input to the build path, and the engine never sees it.

`BuildResult` and `RepositoryResult` are **engine-side only**, and neither crosses this
boundary. `BuildResult` may carry a live `*dagger.Container`; `RepositoryResult` is scalar
because its facade closes the session before returning. The worker persists Observer
callbacks as `build_events`, not either result struct, and nothing srv-side is typed in
terms of them.

Persistence records **intent and observation, never runtime machinery**. The manifest and
selected modules preserve what the trigger requested; the engine still interprets them
through current framework code and never serializes or replays `framework.BuildUnit` or a
stored execution plan. Build hooks remain deferred.

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

- A user with read access can inspect a repo; write access is required to onboard it,
  trigger a build, or retry one.
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
`DATABASE_URL`, the listen address, `SECRET`, and the `DAGGER_ENGINE` seed
([installation.md](installation.md)). This is a *server install* concern owned by the
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
- **The client has one transport layer.** Auth-header selection, JSON decoding, error
  shaping, and pagination are written once, in the transport; each endpoint method
  declares only its path, verb, and response type. Pagination follows the `Link`
  response header — the walk ends when a response carries no `rel="next"`, following
  the header's own URLs (never constructing page URLs) — so there is no page cap:
  the end-of-list signal is the protocol's, not a guessed bound
  (docs/vendor/github-app-api.md §Pagination). Whether the App is installed on an org
  is the direct `GET /orgs/{org}/installation` lookup (404 = not installed), never a
  walk of the installation list.
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
- **User-token freshness** — every successful OAuth login replaces the encrypted stored
  user token. GitHub controls that token's own expiry policy; platform's two-hour login
  session independently bounds how long the browser may act before reauthentication.
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

## Repository source for server builds

The job mints the installation token and supplies immutable repository facts, the selected
module, work id, publish intent, and credentials to engine's remote-build facade. Engine
owns repository preparation, config loading, unit interpretation, runner placement,
execution, publication, lifecycle reporting, and cleanup. The clone/cache mechanism and
layout are specified in [engine.md](engine.md), §Repository preparation; srv never invokes
that mechanism directly.

## Sequencing

Each layer consumes the one below *after* it works. The CLI delivery path, the `srv`
wrap (webhook ingest, auth, and the build pipeline), the App API client, org-owner claim,
credentialed clone, repository registration, whole-repository manual trigger,
repository/build reads, build detail/steps, and truthful `/builds/{id}` status have
shipped. Repository onboarding presents and persists the resolved server publish policy.
The intended server surface is not complete: manual module selection is absent from the
stored build and client request, no read resolves a ref and manifest before queueing, build
events do not record engine attribution, engine reads do not exist, and repository/engine
dynamic routes have no truthful fallback classifier. The repository landing, onboarding,
and System pages have real client reads in source, but the live product does not yet present
the complete repository experience; the repository build feed, manual trigger, build
detail, and engine pages remain mocks.

The next planning pass maps the complete CI/CD experience over `platform srv` and its
existing tooling into implementation slices. It starts from the live product experience,
not from the completed builder internals, and includes the forthcoming cluster-view
capability without pre-cutting that work here. Cluster installation and delivery already
live in the `prod9/infra` GitOps repo; platform deploys nothing — publish pushes the image
and Flux pulls.

## Open details (not blockers)

- Where the `init` server marker lives — `platform.toml` `[server]` field vs CLI-global
  config.
