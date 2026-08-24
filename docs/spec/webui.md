# Webui

Status: accepted intended surface; implementation is partial. The repository landing,
repository-onboarding, and System pages have real client reads in source; onboarding
presents the resolved server publish policy, and registration persists it. The repository
build feed, manual trigger, build detail, and engine pages remain mock presentations, and
the live product does not yet deliver the complete repository experience this spec
describes.
The supporting server surface is partial too: repository/build reads, whole-repository
manual triggers, build detail, and steps exist; pre-queue ref/module resolution, manual
module selection, engine reads, engine attribution, and truthful repository/engine
dynamic-route classification do not. Of the four planned shared components, only the
outcome mark exists.

The webui is the platform server's front end: a SvelteKit app built with
`adapter-static`, prerendered into `webui/build/` and embedded into the `platform`
binary ([platform-server.md](platform-server.md), "`webui/build/` is committed" and "The
status of a page is the server's answer"). This file specs the **product surface** — the
pages a signed-in user works in. The install wizard is specced separately in
[installation.md](installation.md) §The wizard UI.

## Navigation

Three sections: **Repositories · Engines · System.** The repositories page is the
landing page — builds are not a top-level section because a build belongs to a repo, and
the pages nest the same way. The left rail carries the wordmark only, and the wordmark
goes home. Pages run full-width and carry no explainer copy — the UI states facts, it
does not introduce itself.

## Route map

| Route                            | Page                                    | Reads                                                                  |
|----------------------------------|-----------------------------------------|------------------------------------------------------------------------|
| `/`                              | repos landing (+ the sign-in door)      | `GET /api/repos`, fan-out `GET /api/repos/{owner}/{repo}/builds?limit=3` |
| `/repos/new/`                    | repo onboarding wizard                  | `GET /api/repos/candidates`, `GET /api/repos/{owner}/{repo}/manifest`; confirms with `POST /api/repos` |
| `/repos/{owner}/{repo}/`         | one repo's build feed                   | `GET /api/repos/{owner}/{repo}/builds`                                 |
| `/repos/{owner}/{repo}/builds/new/` | manual-trigger wizard                | ref→sha + module read; queues with `POST /api/builds`                  |
| `/builds/{id}`                   | build detail: navigator + terminal      | `GET /api/builds/{id}`, `GET /api/builds/{id}/steps`                   |
| `/engines/`                      | engine fleet                            | `GET /api/engines`                                                     |
| `/engines/{addr}`                | one engine instance                     | `GET /api/engines/{addr}`                                              |
| `/system/settings/`              | System / Settings                       | `GET /api/system/settings`                                              |
| `/system/migrations/`            | System / Migrations                     | `GET /api/system/migrations`; runs `POST /api/system/migrations`        |

Build detail stays `/builds/{id}` — the id is global, and a build link must survive
being pasted without its repo context.

Every dynamic route here is a shape `srv`'s fallback classifier must know
([platform-server.md](platform-server.md), "The status of a page is the server's
answer"): `/builds/{id}`, `/repos/{owner}/{repo}/…`, and `/engines/{addr}` all serve the
fallback at the status the record deserves. Only `/builds/{id}` is implemented today.
The mock tree stands in with static paths (`/builds/`, `/engines/instance/`); the dynamic
shapes above are the target.

## Session expiry and return

Authentication recovery is a shell concern, not a page concern. The shared server client
intercepts `401` from every product API operation and starts GitHub OAuth; a page never
renders an expired-session response as its own loading or mutation error. Before leaving,
the client captures the current same-origin path, query, and fragment. The server binds the
path and query to OAuth state, the client preserves the fragment, and successful login
returns the user to the exact interrupted location with a fresh two-hour session.

Only `401` starts reauthentication. Offline responses, `403`, `404`, conflicts, and server
failures remain page-level outcomes. One recovery may be in flight at a time, so concurrent
reads cannot create redirect loops. A cancelled or failed OAuth attempt returns to a
dedicated sign-in state carrying the preserved destination and a retry action; it does not
automatically redirect again.

## Pages

**Repos landing (`/`).** One block per registered repo — the nested-feed shape: the
repo's name heads the block, its last three builds render as sub-rows (outcome mark,
tag, resolved sha, when), and the block links into the repo's feed. An "add repository"
action leads to the onboarding wizard. Signed-out, the page is the sign-in door and
nothing else.

**Repo onboarding (`/repos/new/`).** Runs as the install wizard does: a checklist on the
left is the navigation, the selected step's action renders beside it, and operative
instructions render on the right. The wizard separates three concerns:

1. **Repository access.** Pick from a clickable, filterable list containing only repos
   reachable by both the signed-in GitHub user and the App. The instructions explain that
   intersection and direct a user whose repo is missing to verify their own GitHub access,
   then the App installation's repository access in the organization settings.
2. **Load `platform.toml`.** Read the selected repo's default-branch manifest as its own
   step. An accessible repository without the file stays here with instructions to run
   `platform init` in the repository and commit the result, or author and commit
   `platform.toml` manually. Repository access failure remains a separate error.
3. **Review and confirm.** Present the resolved commit sha, modules, framework detections,
   and resolved `[server].publish` policy. Policy is stated as behavior: which successful
   builds publish and which image tag they receive, not merely the config value. The
   confirm sends the reviewed sha, not manifest content; the server re-reads the immutable
   commit and atomically stores the repository, exact raw manifest, parsed policy, and
   parsed modules.

Registration model: [platform-server.md](platform-server.md) §Repos are registered,
authorization is GitHub-derived.

**Repo build feed (`/repos/{owner}/{repo}/`).** The repo's builds as a CI feed: newest
first, each row led by its outcome mark, carrying the tag, the commit it resolved to,
per-module marks, and the trigger's provenance (who or what asked). A "new build" action
leads to the manual-trigger wizard.

**Manual-trigger wizard (`/repos/{owner}/{repo}/builds/new/`).** A ref in, its sha
resolved server-side and shown, the modules read from `platform.toml` at that commit and
selectable — all on by default; queueing posts `POST /api/builds`. Webhook builds stay
whole-repo; selection is the manual trigger's alone
([platform-server.md](platform-server.md) §Triggering a build).

**Build detail (`/builds/{id}`).** Two instruments. The **navigator**: facts up top,
then modules and steps on one shared three-column grid — mark gutter, name, time — so
every row aligns whatever its depth. The **terminal**: the selected step's captured
output in a night-ground log pane. The page polls while the build is live; retry
re-posts the same repo + ref as a new build.

**Engine fleet (`/engines/`).** The fleet as the same nested feed the repo list uses:
one block per resolved instance — reachability leading the header, its facts and current
work as sub-rows — each linking to the instance page. A refresh button re-reads; an info
plate states where the roster comes from (the `DAGGER_ENGINE` DNS name, resolved per
request — [engine.md](engine.md) §Runner discovery). Per instance the server reports
what it can honestly know: reachability (dial check), engine version, and current work
(modules whose `engine_host` names this engine and whose events have no `run_done`).
Uptime and cache size appear in the walked design but have **no verified source** — they
enter the wire shape only when a Dagger introspection query is verified and cribbed to
`docs/vendor/`, and are omitted until then.

**Engine instance (`/engines/{addr}`).** The instance's facts, and the builds it has
carried — read back from `build_modules.engine_host`. The walked design also shows a live
engine-log terminal; engine logs are pod logs, which is k8s ground — that pane belongs
to the **cluster-view slice** ([platform-server.md](platform-server.md), the cluster
view held for its own design pass) and is out of this surface.

**System** is one top-level destination with two peer subviews. Its shared subnavigation
keeps Settings and Migrations visible on both pages.

**System / Settings (`/system/settings/`).** The install-time facts, read-only, grouped in sections
(server, GitHub App, registry). Secrets render as middot runs — the server already
serves them masked and never the value (`GET /api/system/settings`). Changing an install fact
is not in this surface; those sections state, they do not edit.

**System / Migrations (`/system/migrations/`).** `GET /api/system/migrations` returns the
ordered fx migration plan, rendered one item per line with its action and migration
name. An empty plan says the schema is current. `migrate` lines enable **Run
migrations**; a new release shipping a migration is remedied here, inside the product
composition, never by demotion to the install wizard
([installation.md](installation.md) §Boot composition). `update sql` (resync) and
`remove` (prune) actions are classified by the client and render as warnings. Their
presence removes the run button and gives one manual recovery instruction:
shell into the server and run `./platform srv data resync-migrations --force`, then
refresh. A failed read renders its error; no separate reachability probe is built, the
read failing *is* the reachability signal.

## Shared components

Four pieces repeat across the pages and are extracted as components when wiring lands
(named in each mock's ⚠ MOCK note): the **outcome mark** (✓ ✗ ◌ · for
succeeded/failed/running/queued), the **feed row** (mark-led row with trailing
timestamp), the **kv list** (muted key, ink value — the settings and facts shape), and
the **terminal pane** (the night-ground log view the build detail and the cluster-view
slice's log reads share).
