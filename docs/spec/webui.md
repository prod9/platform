# Webui

Status: accepted intended surface; implementation is partial. The repository landing,
repository-onboarding, and System pages have real client reads in source; onboarding
presents the resolved server publish policy, and registration persists it. The complete
repository feed and manual-trigger wizard remain the product target; build detail uses
recorded results and step output, while engine pages remain mock presentations. The live
product does not yet deliver the complete repository experience this spec describes.
The supporting server surface is partial too: repository/build reads, manual triggers with
module selection, build detail with engine attribution, and steps exist; pre-queue
ref/module resolution, engine reads, and the complete repository/engine route surface
remain later work. Shared visual primitives (`PageHeader`, `SectionHeader`, `Facts`,
`SignalMark`, and `OutcomeMark`) exist; shared feed rows remain unbuilt. The build-detail
terminal owns presentation of captured step output.

The webui is the platform server's front end: a SvelteKit app built with
`adapter-static`, prerendered into `webui/build/` and embedded into the `platform`
binary ([platform-server.md](platform-server.md), "`webui/build/` is committed" and "The
status of a page is the server's answer"). This file specs the **product surface** — the
pages a signed-in user works in. The install wizard is specced separately in
[installation.md](installation.md) §The wizard UI. Human-facing semantics shared with the
CLI follow [interfaces.md](interfaces.md).

## Navigation

Three sections: **Repositories · Engines · System.** The repositories page is the
landing page — builds are not a top-level section because a build belongs to a repo, and
the pages nest the same way. The left rail carries the wordmark only, and the wordmark
goes home. Pages run full-width and carry no explainer copy — the UI states facts, it
does not introduce itself.

## Visual composition

Shared visual structure is owned by components, never coordinated by route-local CSS.
`PageHeader` owns the title, metadata, and action positions at the top of every product
page. Its row has one stable height, independent of whether metadata or actions are
present, so every page title occupies the same vertical position. Text metadata shares
the title baseline, and navigation labels share that same baseline. The title's
symmetrically padded line anchors the baseline within the fixed header row; navigation
retains its padded click targets without changing the title position. Actions remain
vertically centered. `SectionHeader` owns
the corresponding heading and action rhythm within a page.
`Facts` owns definition-list columns, key/value typography, and their shared vertical
rhythm; its named measures cover the three established key widths. `SignalMark` owns the
glyph, color, dimensions, and accessibility behavior for navigation and state signals;
`OutcomeMark` maps build-domain outcomes onto it.

Routes own only domain-specific composition such as wizard columns, navigator/terminal
splits, responsive collapse, and the space between distinct sections. They do not restyle
the shared primitives or reproduce their markup. A styling-only change preserves every
visible word, value, state, behavior, and data/API contract; layout and iconography may
change without rewriting the content they present.

### v2027 visual system

Status: implemented; first-screen and full-route browser review use isolated API fixtures.
The visual system follows
PRODIGY9's settled v2027 sources, replacing the previous styling throughout `webui/`.
Installation, session recovery, product pages, existing mock pages, and their loading,
empty, refused, offline, stale, selected, and disabled states share this system.
Mock pages retain their existing capabilities.

The palette is paper `#fafaf5`, soft `#eef2f3`, ink `#1c1f33`, muted `#636c81`,
hairline `#dfe5ea`, indigo `#3a4d98`, secondary indigo `#4f63a9`, bike red `#e2282e`,
and night `#121526`. The monocolor logo, primary action, and lit tags use the same bike red.
Paper is the page ground; night is contextual, including captured-output panes.
Application adaptation: success retains a distinct accessible glyph, running work uses
indigo, pending work uses muted ink, and failures use red. This treatment is subject to
the first recorded build-detail screen's visual review, not a settled site brand ruling.
Status meaning remains readable without color.
Text and focus indicators must contrast with their actual surface.

Iosevka Curly owns headings and names, Iosevka Aile owns body text, Iosevka owns data,
and Mona Sans Extra Expanded owns buttons. Type follows the v2027 perfect-fourth scale,
with line heights and spacing on a 4px unit. Operational screens adapt the site's
proportions for readable facts, feeds, forms, and logs while keeping full-width pages.

Controls use native semantics, soft fields, logo-angle chamfers (rise 1.48), visible
focus, and crisp 3px depth with pressed and selected states. Raised panels and inset
separators establish hierarchy. Leaning parallelograms are reserved for tags inside
night caps. Selected Akira masks may texture caps at 6% strength with a 1.25 cap gain;
technical values and output remain unobstructed. Reduced motion preserves state cues
without animated travel. Narrow layouts wrap long values and collapse domain columns
without clipping content or focus indicators.

Shared components own this presentation; route-local styles own domain layout only.
Replace previous rules directly, without legacy token aliases or an override skin.
Fonts, logo masters, and selected art are self-hosted in platform, with source paths
and revision recorded; neither runtime nor build depends on the sibling checkout.
Regenerate the committed embedded `webui/build/` with the production UI build.

Chakrit explicitly ruled: "remove EVERYTHING from the old design ok? including mode
toggles." Remove the old tokens, styling, assets, unused dependencies, theme module,
preference reads/writes, and mode toggle labels. There is no replacement theme toggle.
This is the sole exception to the cosmetic preservation contract above; all other
visible words, data, actions, state distinctions, and API contracts are preserved.

Source provenance: `prod9/prodigy9.co` revision
`04ce0e59e55a8fc7963e50cbc542994aefbf7b90`, `src/styles/{tokens,type,diagonals,depth,
controls,components,texture}.css`, `src/components/preview/{akira-logo,nav}/meta.js`,
`logo_bundle/akira/transparent/wordmark-red.png`, `public/fonts/`, and
`public/art/akira-cap-full-1.webp`. The site's Outline
collection remains its authority; connector access was unavailable during the local
source review. Any subsequently discovered conflicting ruling must be reconciled before
adopting the affected presentation. Earlier brand defaults do not supersede v2027.

## Route map

| Route                            | Page                                    | Reads                                                                  |
|----------------------------------|-----------------------------------------|------------------------------------------------------------------------|
| `/session/`                      | absent or failed login session          | none                                                                   |
| `/installation/`                 | first-install wizard                    | `GET /api/installation`; updates its installation subresources         |
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
fallback at the status the record deserves. The first-build slice adds the repository
root route alongside `/builds/{id}`; nested repository and engine routes remain later work.
The mock tree stands in with static paths (`/builds/`, `/engines/instance/`); the dynamic
shapes above are the target.

## Session expiry and return

During startup checks, the shell renders the destination page's own loading chrome with
interaction disabled. Page reads wait until installation routing and the session read
finish; the landing page shows its sign-in door only after the session is known. Unknown
data uses static placeholders inside the destination's existing panels and controls,
including the repository manifest source panel. Loading controls are disabled controls,
with no pointer or hover affordance.

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

Every explicit GitHub sign-in action enters that same one-in-flight state synchronously on
its first activation. The action immediately disables and reads **Signing in…**; another
activation is refused before the browser leaves the page. A failed or cancelled OAuth flow
returns as a fresh `/session/` document, whose retry begins idle again.

That state is `/session/`. Its retry targets `/auth/github` with the preserved same-origin
path and query; the browser keeps the fragment in session storage under the destination so
only the matching successful return restores it. During installation, the sign-in URL also
carries the App `installation_id` from the setup landing URL because no bound installation
record exists yet.

## Pages

### Bounded repository-history slice

The repository page at `/repos/{owner}/{repo}/` presents the latest 50 builds as a
compact feed, newest first. This extends the initial whole-repository build trial into
usable recent history; the manual-trigger wizard remains a separate increment. The page
states the latest-50 bound and offers no pagination or promise of older history. The
landing page keeps its independent three-build preview per repository.

The repository page uses the existing `GET /api/repos/{owner}/{repo}/builds?limit=50`
and `POST /api/builds` operations. It retains an initially empty ref field with an example
such as `refs/heads/main`; the operator enters a ref and **Start build** sends `owner`,
`repo`, and `ref`, omitting `modules` so every module at the server-resolved commit is
selected. The page disables submission until the session and repository read have
completed, and while a request is in flight. It never invents a default branch.

A successful response appears immediately on the same page with accepted-build feedback
and its actual detail link. Insert that build into the newest-first history, replacing a
matching id and retaining at most 50 entries. Build rows show their id, canonical status
with supplementary outcome mark, exact ref, full SHA, and recorded source timestamps.
Created time is always shown; started and finished times appear only when recorded.
Abbreviated SHAs, relative times, and durations may supplement those values, never replace
them. The trigger and numeric `user_id` identify recorded provenance with their exact
values and explicit labels; do not infer an actor name or translate a principal id into
one. Each row links to the recorded build-detail page.

Module sub-rows retain API order and show every selected module's name, canonical status,
and recorded error. Build errors remain visible too. Publication image and hash appear
only when present in the response. Long refs, SHAs, module names, errors, and image values
wrap without clipping essential facts or making the page require horizontal scrolling.

The landing preview and repository history share feed-row presentation and outcome/time
primitives; one component owns their row spacing, responsive structure, and link/focus
treatment. Each page retains its own source-value contract: sharing presentation does not
add canonical status labels or full timestamps to the landing preview. Reads, mutations,
and snapshot state remain with the page that owns them.

**Refresh** reads current build states without overlapping requests; the bounded slice
adds no polling, step-output pane, module picker, pre-queue resolution, or retry action.
Loading, empty, refused, and offline outcomes stay distinct; a failed refresh retains
the last successful read with an explicit stale-state message. A failed submission never
creates a success message or an automatic second request.

The server recognizes exactly `/repos/{owner}/{repo}` with an optional trailing slash,
applies the installation and repository-read gates, then checks registration before
serving the SPA shell. Missing or inaccessible repositories return `404`; a missing
session returns `401`; unexpected lookup failures return `500`. Extra path segments
remain unknown routes. No database migration or new API operation is needed.

### Recorded build-detail slice

`/builds/{id}` reads the actual route id through `GET /api/builds/{id}` and
`GET /api/builds/{id}/steps`; no sample build, step, actor, attempt, or output is shown.
Repository build rows link to this route, and its repository heading links back to the
repository page. The page waits for the shell's session checks before reading.

The header and facts show the canonical build id, repository, status, ref, full SHA,
trigger, user id, and recorded timestamps. Retry ancestry is a fact when present, not a
retry action. Modules retain their API order, including queued modules with no steps;
their results include errors, image/hash, worker claim and engine attribution, and last
recorded event when available. Empty strings and Go zero times do not become invented
values or durations.

The navigator groups the flat recorded steps by build-module id, preserving their start
order and separating identical step names in different modules. Closed steps show
succeeded or failed according to their recorded error. Unclosed steps show running while
their module is nonterminal and incomplete after a terminal module result; neither case
invents a finish time. Selection identifies the module and the step's name and start time,
survives refresh while that observation remains present, and initially chooses the first
recorded step. No steps yet is distinct from missing output for a recorded step.

The terminal presents captured stdout and stderr as separately labeled, escaped text,
with the recorded step error alongside them. Separate buffers do not imply chronological
interleaving. Empty output is stated honestly; output is available only after the server
has persisted it, and this page adds no live streaming.

**Refresh** re-reads build results and steps without overlapping requests. Refused,
offline, initial-loading, and empty outcomes stay distinct. A failed refresh retains the
last successful snapshot with a stale-state message. Partial reads must not present new
metadata alongside old step output as a fresh snapshot. Route changes discard prior
build data and outstanding results. This slice adds no polling or retry control; the
complete target below retains those as later work.

The HTML route and both API reads share the build fragment's authorization and lookup
boundary: a missing session returns `401`, and a missing or inaccessible build returns
`404`. No new API operation or database migration is required.

**Repos landing (`/`).** One block per registered repo — the nested-feed shape: the
repo's name heads the block, its last three builds render as sub-rows, and the block links
into the repo's feed. The page heading and its registered-repository count share a text
baseline. Each build shows its ref and resolved sha. Status is the outcome mark alone, not
a repeated text label. Its most recent source timestamp is presented as
`finished HH:MM:SS`, `started HH:MM:SS`, or `created HH:MM:SS`; storage field names and
full timestamps do not leak into the feed. A "register repository" action leads to the
onboarding wizard.
The shared [interface draft](interfaces.md#web-ui) proposes canonical source values
remaining visible beside every derived mark or timestamp; reconciling that proposal with
this landing-page presentation remains an unresolved design question.
Signed-out, the page is the sign-in door and nothing else.

**Repo onboarding (`/repos/new/`).** Runs as the install wizard does: a checklist on the
left is the navigation, the selected step's action renders beside it, and operative
instructions render on the right. The wizard separates three concerns:

1. **Repository access.** Pick from a clickable, filterable list containing only repos
   reachable by both the signed-in GitHub user and the App. The instructions explain that
   intersection and direct a user whose repo is missing to verify their own GitHub access,
   then the App installation's repository access in the organization settings.
2. **Load `platform.toml`.** Read the selected repo's default-branch manifest as its own
   step. A successful read shows the exact raw file in this step before Review is
   available. An accessible repository without the file stays here with instructions to
   run `platform init` in the repository and commit the result, or author and commit
   `platform.toml` manually. Repository access failure remains a separate error.
3. **Review and confirm.** Present the resolved commit sha, modules one line per module
   with its framework, framework detections, and resolved `[server].publish` policy.
   Present `publish_policy` under a human-readable label kept close to the field name, such
   as `Publish policy`, and preserve the exact resolved enum value. A relevant derived
   value appears as a hint on that field, never as a translated peer field. The confirm
   sends the reviewed sha, not manifest content; the server re-reads the immutable commit
   and atomically stores the repository, exact raw manifest, resolved `publish_policy`,
   and parsed modules.

Registration model: [platform-server.md](platform-server.md) §Repos are registered,
authorization is GitHub-derived.

**Repo build feed (`/repos/{owner}/{repo}/`).** The repo's builds as a CI feed: newest
first, each row led by its canonical status and supplementary outcome mark, carrying the
ref, full commit sha, source timestamps, per-module statuses, and the trigger's
provenance (who or what asked). Recognizable tags, abbreviated shas, relative times, and
durations are hints on those source values. The bounded repository-history slice above
implements the latest 50 with the whole-repository Start build form. A New build
action leads to the reviewed manual-trigger wizard.

**Manual-trigger wizard (`/repos/{owner}/{repo}/builds/new/`).** The repository page
offers a New build link alongside its retained whole-repository Start build form.
The wizard resolves an entered ref through the manifest resource's `?ref=` read and
shows the full resolved SHA, exact raw manifest, policy, and modules from that commit.
All modules are initially selected; explicit empty selection cannot be submitted.
Ref edits invalidate the entire review and selection until the new ref is loaded.

Queueing posts owner, repo, ref, selected module names, and `reviewed_sha` to
`POST /api/builds`. A moved ref returns `409`; the wizard discards its review, states
that the source changed, and requires loading/reviewing it again. It never queues a
different source silently or automatically resubmits after a failure. While loading
or queueing, competing actions are disabled. Success opens the returned actual build
detail. Session expiry, inaccessible repo/ref, absent/invalid manifest, transport failure,
and empty modules have honest distinct states; stale async results after ref changes
or navigation cannot repopulate the review. The nested HTML route shares repository
read authorization and registration lookup with its repository root.

Webhook builds remain whole-repository; selection is the manual trigger's alone
([platform-server.md](platform-server.md) §Triggering a build). This review operation
creates no build, registration, snapshot, or job.

**Linked retry.** Build detail offers Retry for a readable recorded build of any status.
It opens `/repos/{owner}/{repo}/builds/new/?retry_of={id}` using the same reviewed
manual form. The form first reads the authorized predecessor, verifies its repository
matches the route, and fixes the ref to the predecessor's exact ref. Missing,
inaccessible, malformed, or mismatched predecessors cannot become ordinary new builds.
Show a link to the predecessor and explain that the ref is resolved again; retry is not
a replay of its old SHA and does not cancel its work.

Loading source reviews the current commit and manifest. Initially select only modules
whose names appeared in the predecessor and still exist in the reviewed manifest;
new modules remain unselected. Explicitly list predecessor modules that disappeared so
the operator can review the difference and intentionally choose the new selection.
An empty selection disables queueing. Queueing sends `retry_of` with the reviewed SHA
and chosen modules. The existing moved-ref recovery and single-pending-request behavior
apply. Success opens the new build, whose detail links back through `retry_of`; history
retains both records. A different ref requires the ordinary New build flow.

**Build detail (`/builds/{id}`).** Two instruments. The **navigator**: facts up top,
then modules and steps on one shared three-column grid — mark gutter, name, time — so
every row aligns whatever its depth. The **terminal**: the selected step's captured
output in a night-ground log pane. The recorded-detail slice above implements explicit
Refresh. Polling while a build is live and retrying the same repo + ref as a new build
remain outside that slice.

**Engine fleet (`/engines/`).** The fleet as the same nested feed the repo list uses:
one block per resolved instance — reachability leading the header, its facts and current
work as sub-rows — each linking to the instance page. A refresh button re-reads; an info
plate states where the roster comes from (the `DAGGER_ENGINE` DNS name, resolved per
request — [engine.md](engine.md) §Runner discovery). Per instance the server reports
what it can honestly know: current-roster connection/version observations and recorded
unfinished assignments. Fetch `GET /api/engines`; show the configured source, inspection
time and roster state, with explicit Refresh and honest unconfigured/empty/lookup-failed,
partial-endpoint, and stale-read states. A failed refresh retains its previous snapshot
with a stale label. Show assignments with their source ref/SHA, recorded status and
observation time, linking only to authorized actual build details. The page states that
unfinished records do not prove live execution or engine idleness.
Uptime and cache size appear in the walked design but have **no verified source** — they
enter the wire shape only when a Dagger introspection query is verified and cribbed to
`docs/vendor/`, and are omitted until then.

**Engine instance (`/engines/{addr}`).** The instance's facts, and the builds it has
carried — read back from `build_modules.engine_host`. Its `GET /api/engines/{addr}` read
shows the latest 50 authorized endpoint assignments, source values and actual build
links. Current, unreachable, version-unavailable, historical-only absent, discovery-
unknown, and SDK-managed `local` observations are distinct; unknown identities are 404.
The page identifies history as endpoint attribution, not a guarantee of physical engine
continuity across address reuse. Refresh is explicit and preserves labeled stale data.
Replace the static instance mock with the real dynamic route; frontend links URL-encode
the exact reported address. The walked design also shows a live
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

## Observation freshness

Repository module rows, build-detail modules, and engine assignments consume the server's
`observation` object. A `quiet` observation shows "No recent observations" beside its
exact `last_observed_at`; retain the canonical execution status and recorded source
timestamps. A shared presentation owns this indication. Queued/unclaimed, recent, and
terminal work do not receive the quiet warning. A quiet step can still be running; the
UI never calls it dead or abandoned. Refresh updates the indication from the new server
evaluation, with the same stale-snapshot rules as the enclosing view. No timer, polling,
automatic retry, or new execution status is added by this indication.

## Shared components

Four pieces repeat across the pages. The **outcome mark** is implemented by `OutcomeMark`
over `SignalMark`; the **kv list** is implemented by `Facts` (muted key, ink value — the
settings and facts shape). The **feed row** (status-led row with its source timestamp and
optional derived hints) is owned by `FeedRow`. `TerminalPane` owns the build detail's labeled
stdout/stderr buffers and recorded error, with escaped output in a night-ground log view.
The cluster-view slice's live log reads remain deferred. Mock notes describe pending
pieces rather than overriding the implemented shared primitives.
