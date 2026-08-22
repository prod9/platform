# Webui

Status: accepted — designed 2026-08-12; the product routes currently carry this design
over canned data (⚠ MOCK notes in each page), wiring lands next.

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
fallback at the status the record deserves. The mock tree stands in with static paths
(`/builds/`, `/engines/instance/`); the dynamic shapes above are the target.

## Pages

**Repos landing (`/`).** One block per registered repo — the nested-feed shape: the
repo's name heads the block, its last three builds render as sub-rows (outcome mark,
tag, resolved sha, when), and the block links into the repo's feed. An "add repository"
action leads to the onboarding wizard. Signed-out, the page is the sign-in door and
nothing else.

**Repo onboarding (`/repos/new/`).** Runs as the install wizard does: a checklist on the
left is the navigation, the selected step's action renders beside it. Step one picks the
repo from a clickable, filterable candidate list; step two reviews what the server
pre-read from the repo's `platform.toml` — modules, framework detections — and carries
the confirm. Registration model:
[platform-server.md](platform-server.md) §Repos are registered, visibility is live.

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
then attempts and steps on one shared three-column grid — mark gutter, name, time — so
every row aligns whatever its depth. The **terminal**: the selected step's captured
output in a night-ground log pane. The page polls while the build is live; retry
re-posts the same repo + ref as a new build.

**Engine fleet (`/engines/`).** The fleet as the same nested feed the repo list uses:
one block per resolved instance — reachability leading the header, its facts and current
work as sub-rows — each linking to the instance page. A refresh button re-reads; an info
plate states where the roster comes from (the `DAGGER_ENGINE` DNS name, resolved per
request — [engine.md](engine.md) §Runner discovery). Per instance the server reports
what it can honestly know: reachability (dial check), engine version, and current work
(builds whose events name this engine and have no `run_done` yet). Uptime and cache
size appear in the walked design but have **no verified source** — they enter the wire
shape only when a Dagger introspection query is verified and cribbed to
`docs/vendor/`, and are omitted until then.

**Engine instance (`/engines/{addr}`).** The instance's facts, and the builds it has
carried — read back from `build_events.engine`. The walked design also shows a live
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
