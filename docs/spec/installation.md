# Installation

Status: **built.** The installer fragment, the `GET /api/install` state surface,
the migrations, App-creation, and credentials actions, the boot-composition gating, the
org-owner claim, and the wizard install page
(`webui/src/routes/install/+page.svelte`) all ship today (`srv/install`,
`srv.Router`). The auth model this sits on is frozen in
[platform-server-github-app-zero-rbac](../decisions/2026-06-29-platform-server-github-app-zero-rbac.md);
the route surface lives in [platform-server.md](platform-server.md).

## What installation is

A platform server governs **one GitHub org**. Installation is the one-time act
of pointing a fresh server at that org: running migrations, naming the server's
public URL, creating the GitHub App, supplying the registry push token, and
installing the App. Until all of that is true, the server is **not completely
installed** and serves only the installer. (The org-wide delivery webhook is
cluster-side plumbing outside the wizard — §The org-wide GitHub→Flux delivery
webhook.)

The install flow is a **wizard**: the webui walks the state entries as
step-by-step guided pages, and every value the operator enters — the App
credentials, then the org binding — is saved into the settings app as its step
completes, advancing the wizard to the next step. **All install-time settings
live in settings**; the deployment supplies only `DATABASE_URL`, the listen
address, and `SECRET` (the fx encryption key — it cannot live beside the
ciphertext it protects) through fx config/env. srv refuses to boot the
**installed** composition with `SECRET` unset — fx would otherwise silently
derive a publicly known key. The deployment also sets
`DAGGER_ENGINE` — plain runtime config on both srv and the worker: the Dagger
engine pool's headless-Service DNS name, spread across pods by k8s DNS itself,
so no engine binding is stored server-side.
The worker clones into the fixed `/var/cache/platform` (not configurable — the
deployment mounts a writable volume there).

The wizard UI holds these rules:

- **A step's action completes that step.** Clicking a step's button ends with
  that entry `fully_ready` and the wizard advanced — never a return to the same
  step with more to do. Work that can't satisfy that in one action is two
  steps, not one panel with phases (the install/claim split is the canonical
  case).
- **Progress is always visible, and it is navigation.** The full ordered state
  list renders on every step, statuses as returned by `GET /api/install`, and
  every entry is clickable — opening that step's panel. The **default**
  selection (on load and after every save) is the first non-`fully_ready`
  entry, so doing nothing but following the default walks the install in
  order; navigation exists so a done step can be revisited and redone.
- **One panel at a time.** Below the progress list sits exactly one step panel
  — the selected entry's.
- **Each panel is operative.** It carries the detailed instructions the human
  operator follows, direct links to the external pages the step works on (the
  GitHub App creation page, the created App's edit and install pages — all
  built from the org and app-created entries' `values`), and — where the step
  takes values — the input fields and a save action posting the step's
  installer action. Non-secret fields pre-fill from the entry's `values`;
  secret fields always render empty.
- **A form is editable only when its values are settled.** The wizard renders
  nothing until the first state read has been adopted, and a panel's inputs are
  disabled while its own save is in flight — adopting a state response must
  never replace text the operator is still typing, and both windows where that
  could happen are closed by construction rather than reconciled after.
- **Done locks; Redo unlocks.** A `fully_ready` step's panel renders its form
  locked behind a **Redo** button. Redo is client-side only — it unlocks the
  form (secret fields empty and required; the webhook secret, being generated
  rather than recalled, re-mints fresh on every redo); the server learns
  nothing until the save lands, which then suffix-resets every later step
  (§Redo and suffix invalidation).
- **Restartable, always converging.** The wizard holds no step state of its
  own: every save's response (and every page load) is a fresh state read, so a
  reload, a failure, or out-of-order work always lands the operator back on
  the first unfinished step, with the progress list telling the truth.

The whole concern lives in a single **installer fragment** (an fx app fragment).
Product fragments — hooks, builds — have **zero install *flow* awareness**: they
are mounted only once the server is claimed and never ask "am I
installed" — boot answers that exactly once. The **bound install settings are
ambient truth**, though: any product fragment may read them (`install.Load`) —
that is consuming the binding, not install awareness. When HTTP handlers need
it, the install fragment delivers it as request-context middleware (the fx
data-context pattern); until then the worker path reads the settings per job.

**The auth fragment is the exception: it mounts in both compositions.** The
org-owner claim requires a logged-in GitHub user *before* the server is
installed, so login (`/auth/github`, its callback, and the session endpoints)
cannot sit behind the installed gate. Login still has real prerequisites — the
`github.app_*` settings and migrated `users`/`sessions` tables — which the
install order already guarantees before the claim, the only pre-install step that
needs a session. A login attempted earlier fails on those grounds and the install
page never offers it earlier.

## The `GET /api/install` state surface

The installer exposes one read endpoint, `GET /api/install`, returning an
**ordered list of state entries**. Each step is a self-contained wizard unit
(`Step`: `Check` produces the step's whole `Entry`; `Reset` clears the step's
own values — see §Redo and suffix invalidation) and reports one of five
states; the **first non-`fully_ready` entry is the `next` step** — the webui's
default selection (see §The wizard UI).

| State                   | Meaning                                                                       |
|-------------------------|-------------------------------------------------------------------------------|
| `fully_ready`           | the step's condition holds                                                    |
| `partially_ready`       | some but not all of the step's work is done                                   |
| `not_started`           | none of it yet — the step's action is the next move                           |
| `intervention_required` | an operator condition, not a one-click fix (dirty schema, bad `DATABASE_URL`) |
| `""` (unknown)          | the check itself failed — indeterminable, message carries why                 |

Unknown is the **zero value** on purpose: an unset state reads as "nobody
knows", never as a verdict. An entry carries `message` alongside the state when
the check produced an error, and `values` — a string map of the step's
**non-secret form fields** — so a re-opened panel can re-display what is saved
(`server` surfaces the public URL; `org` the slug; `app-created` its app id,
client id, and app slug; the secret-only steps surface nothing). Settings never
round-trip wholesale: the state read is ungated, so the only values that reach
the wire are the ones a step deliberately puts in its own `values`, and secrets
never qualify — a secret's presence is implied by the step's state, never
echoed.

| Entry             | Check                                                                                                                                | Non-ready meaning                                                                                                         |
|-------------------|--------------------------------------------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------|
| `db-reachable`    | connectivity ping                                                                                                                    | `intervention_required` → connection fix                                                                                  |
| `migrations`      | schema is current                                                                                                                    | `not_started`/`partially_ready` → run button; dirty → `intervention_required`                                             |
| `server`          | the `server.public_url` setting has a value; surfaces it in `values`                                                                 | `not_started` → the name-the-server wizard step                                                                           |
| `org`             | the `github.org` setting has a value; surfaces it in `values`                                                                        | `not_started` → the name-the-org wizard step                                                                              |
| `app-created`     | the creation-time values — `github.app_id`, `github.app_slug`, `github.app_client_id`, `github.app_webhook_secret` — all have values | `not_started` → the create-the-App wizard step                                                                            |
| `app-credentials` | every `github.app_*` setting has a value **and the App carries the required permissions**                                            | `not_started` → the generated-keys wizard step; `partially_ready` → App reachable but under-scoped, message names the gap |
| `registry-token`  | the `registry.ghcr.io.token` setting has a value                                                                                     | `not_started` → the registry-PAT wizard step                                                                              |
| `app-installed`   | `GET /orgs/{org}/installation` (JWT) answers 200 for the bound org; its 404 means not installed                                      | `not_started` → install the App on the org                                                                                |
| `claimed`         | every `install.*` setting has a value                                                                                                | `not_started` → sign-in + org-owner claim                                                                                 |

The `server` step names the server's **public URL** — the one server-side truth
of where this deployment lives. It is what the OAuth `redirect_uri` is built
from (login refuses to run without it) and what every wizard instruction that
says "the server's own URL" renders. The webui builds its instructions from the
**server's** value — not the browser's origin — and warns when the two differ: a mismatch usually means
the operator is on a non-canonical host, and values pasted into GitHub from
such a page would point at the wrong place. The panel pre-fills the field from
the browser origin when the setting is empty — a suggestion, not a source of
truth.

The `org` step names the **primary org** — the slug every later panel's GitHub
links are built from (`github.com/organizations/<org>/…`), saved server-side so
any tab or browser renders real links. It sits first among the settings-backed
steps because the wizard is the operator's authority on how to install: the App
is created wherever the org's links point, so everything after depends on it. A
wrong slug is also **self-arresting** — its links 404/403 at GitHub, so nothing
downstream can succeed behind it; the fix is redoing the org step, and suffix
invalidation (below) covers whatever, if anything, existed after. The server
never validates the slug against GitHub — the claim independently derives the
real org from the installation, and only that derived org is install-critical.

The App steps are two because GitHub's flow is two: **creating** the App yields
its id, its slug (the name-derived segment in the App's own URL), and client id
(and echoes back the webhook secret the operator typed into the form), while the
**private key and client secret are generated afterward** on the created App's
settings page. The slug is what later panels build the App's **direct** links
from — its edit page (`github.com/organizations/<org>/settings/apps/<slug>`) and
its install page (`…/settings/apps/<slug>/installations`). Each step saves what
its GitHub page produced, so a reload lands the operator exactly where the
real-world flow stands.

Two steps can be `partially_ready`: `migrations` (some applied, some pending)
and `app-credentials` (credentials saved, but `GET /app` reports a permission
below the required set — the message names exactly which). The claim writes
all-or-none, so it never is. Remediations are **convergent and re-runnable**.

The credentials check compares `GET /app` (JWT auth) against the required set:
`contents: write`, `metadata: read`, `members: read` — `write` satisfies a
`read` requirement. An App whose credentials fail JWT/API entirely is
`unknown` with the error, not partially ready.

The wizard is **complete** when the conjunction holds: **every entry
`fully_ready`**. Completion is the wizard's own notion — what the final panel
waits for — and is distinct from **installed**, the durable fact boot reads
(§Boot composition): once the claim has written the `install.*` record, the
server is installed regardless of what any live check reports later. The
order matters twice over — it is both the wizard's sequence and the
invalidation suffix (§Redo and suffix invalidation): every install-time value
lives in settings, and the settings table exists only once migrations ran — so
**migrations precede every settings-backed entry**; `server` heads the
settings-backed steps (every later panel's "the server's URL" renders from it);
`org` follows (everything after is done on pages its slug locates);
`app-created` precedes `app-credentials` (the keys are generated on the App
that creation produced), `registry-token` follows the App steps,
`app-installed` follows (its check asks GitHub with
the App's own credentials — no session involved), and `claimed` stays last (the
claim needs the installation to resolve and a signed-in org owner).

`app-installed` holds no server-side values at all — its truth lives on GitHub
(the installation exists or it doesn't), the check reads it fresh every time,
and its `Reset` is a no-op: undoing it is uninstalling the App on GitHub, which
the next check simply observes.

## Redo and suffix invalidation

A saved step can be **redone** — same action, new values, convergent as every
remediation is. Because the order is dependency order, new values for step N
make everything after N unreliable (a re-created App orphans its old keys), so
**every save also resets the whole suffix**: the action writes its own step
all-or-none, then calls each later step's `Reset` in the **same transaction**.
`Reset` is the step's own method — each step clears only keys it owns;
settings-backed steps write `""` (an empty value already reads as unset, so
states flip with the plain upsert and nothing needs a delete verb),
`db-reachable` and `migrations` reset nothing (nothing about them is
operator-entered). There is **no reset endpoint and no server notion of "redo
in progress"** — the webui's Redo button is a client-side unlock, and the
server learns of the redo only as an ordinary save.

The rule is deliberately uniform — suffix, not dependency graph. The one
over-invalidation it admits is `registry-token` falling to an App-chain redo
(the PAT is user-scoped and App-independent); re-pasting a token is accepted
as the price of a one-sentence rule.

Saves refuse empty fields, always: an omitted or blank field is never an
upsert of emptiness and never a delete. Redoing a secret-bearing step
therefore means re-entering its secrets — which mirrors reality, since a
secret can't be copied back out of GitHub and a redo regenerates it there
anyway. Explicit value *removal* stays unbuilt; the empty-never-writes posture
is what keeps that door open.

`partially_ready` cannot collide with the locked-form redo UX, recorded here
so it isn't re-derived: all-or-none saves make partial *form input*
unreachable, so the state's only producers are diagnostic — `migrations`
(panel is a button, not a form) and `app-credentials` (App reachable but
under-permissioned; the fix is on GitHub's side and the panel stays open for
a re-check). Only `fully_ready` locks a panel.

**Each check is isolated and install-safe on its own** — no check assumes an
earlier one ran, and none may issue a query it can predict will fail. The
settings-backed checks probe for the settings schema (`to_regclass`) before
reading and report `not_started` when it is absent; the probe always parses, so a
pre-install server never sends a failing statement — retried failing statements
are what Neon's pooled endpoint kills connections over
([vendor/neon-pooling.md](../vendor/neon-pooling.md)). This
tolerate-the-unbuilt-world assumption is the install fragment's alone: product
packages (the App client's settings reads included) treat a missing settings
table as the genuine fault it is post-install.

## Boot composition — the installer gates the product API

Boot decides the API composition **once**, from the **claimed record alone**
(`install.Installed` — every `install.*` setting non-empty, read install-safe:
settings-schema probe first, so a pre-schema database answers "not installed"
rather than eating a failing query). Installed-ness is the durable fact that
the claim completed, **not** the live all-steps conjunction — a server that
was claimed stays in the product composition whatever the checks would say
today:

- **Webui `GET /*` and the auth fragment are mounted unconditionally** in both
  states. They never need remounting.
- **Not claimed** → installer *action* endpoints are mounted; product `/api/*`
  is **not**. `GET /api/install` is served here — it is **part of the gated
  installer fragment, not an always-available endpoint**.
- **Claimed** → product `/api/*` is mounted; the installer actions are gone.

**Post-install conditions never demote the server to the wizard.** A new
release shipping a migration, a database blip, an App permission drifting on
GitHub — these are operational states of an installed server, surfaced and
remedied inside the product composition (the `srv/system` fragment's
`GET /api/settings` and `GET`/`POST /api/migrations` —
[platform-server.md](platform-server.md) §Operations), never by re-entering
the install flow. The wizard's live checks run only while the server is
unclaimed.

The **installer→product transition is a process restart** — boot decides
composition, there is no in-process hot-swap. The restart is self-inflicted:
once the claim's write commits and its response is sent, srv logs that the
install is complete and **exits 0**; the supervisor (k8s) restarts the process,
which boots into the product composition. The wizard's final panel polls
through the blip and lands on the product UI.

**Every installer-composition process converges on that same restart.** Only
one process serves the claim; its peers learn the world changed by re-probing:
a process booted into the installer composition re-reads `install.Installed`
on an interval and, the moment the record reads claimed, takes the identical
exit-0 restart. This is what converges a multi-replica deployment (the replicas
that did not serve the claim) and a process that booted blind because the
database was unreachable — its probe reconnects until the database answers. An
installed process has nothing to converge and runs no probe; the only
installed→installer transition is manual surgery plus a restart, by
convention.

### The SvelteKit SPA drives the installer-vs-app view

The redirect to the installer is **SPA code, not the backend**. The root-layout
guard probes `GET /api/install`:

- **200 + not-installed** → the SPA redirects to `/install` and runs the flow.
- **404** (installer fragment absent) → installed (claimed) → render the app.

`GET /api/install` is deliberately **not always-available** — its presence *is*
the signal. Depending on 404-as-signal is accepted for now.

**Every state read classifies by that signal, not just the root-layout guard.**
The install page's own read applies the same three-way classification: a 404
means the server got installed since the shell loaded, and the page forces a
full navigation to `/` (a fresh shell from the installed composition — never a
client-side route, which would keep the stale shell). Only a genuinely
troubled read — no answer, or a non-404 refusal — renders as a load error.

## First-install gate — no secret, org-owner claim

There is **no install secret**. Install endpoints require GitHub auth, and the
authenticating (first) user must be an **org owner** of the org the server binds
to. That user becomes the seed admin.

No heavier scheme is warranted: platform srv is an internal tool on an
unadvertised domain — its being live is not discoverable, so the org-owner check
is the whole gate. This is consistent with the zero-RBAC model: authorization
stays GitHub-derived, nothing stored.

## Org binding

The org is **set at install**. Changing it is a **de-install + re-install** — the
server binds to exactly the org set at install time and does not rebind live.

## The install settings

Install state lives in **fx's settings app** (`fx.prodigy9.co/app/settings` — the
`settings` key/value table), under two fixed key families. There is no bespoke
table and **no migration defines the keys**: they are hard-coded in the API, a
write upserts, and reading an absent key yields the empty value. The values are
supplied by the wizard — credential steps write the `github.app_*` keys as the
operator enters them; the org-owner **claim** writes the `install.*` keys:

| Key                         | Value                              |
|-----------------------------|------------------------------------|
| `github.app_id`             | the created App's id               |
| `github.app_slug`           | the App's URL slug — direct links  |
| `github.app_private_key`    | PEM private key (PKCS1 or PKCS8)   |
| `github.app_webhook_secret` | webhook HMAC secret                |
| `github.app_client_id`      | user-OAuth client id               |
| `github.app_client_secret`  | user-OAuth client secret           |

`srv/github`'s `LoadApp` reads these settings — **App credentials live in the
database, not fx config**; srv and the worker read the same rows
([platform-server.md](platform-server.md), "Auth mechanism"). Secret values in
Postgres is the allocation config-allocation.md already assigns the server;
encryption at rest remains open there.

**The server binding** rides the same store:

| Key                 | Value                                                                                  |
|---------------------|----------------------------------------------------------------------------------------|
| `server.public_url` | the server's public URL — OAuth redirects, every "the server's URL" the wizard renders |

**The registry token** rides the same store, keyed by registry host:

| Key                     | Value                                           |
|-------------------------|-------------------------------------------------|
| `registry.<host>.token` | classic PAT with `write:packages`, per registry |

ghcr accepts no App-derived credential — classic PATs are the only thing that
works outside Actions ([vendor/ghcr-auth.md](../vendor/ghcr-auth.md)) — so the
token is operator-supplied, a machine-user or org-owner PAT. The wizard's
`registry-token` step writes the `ghcr.io` key; that one key is what the check
requires. Only the token is stored: the publish path pairs it with the
installation record's `install.installed_by_login` as the basic-auth username
(the username question is the vendor doc's one unverified fact). **Punted, on
record:** a wizard UI for additional registries (the key shape already admits
them), and moving the credential to project-scoped settings once projects are a
settings scope — today the rows are server-global, one credential per registry.

The wizard's typed steps post five installer-fragment actions, one per page
the operator works: `POST /api/install/server` writes the public URL
(`server.public_url`), `POST /api/install/org` writes the primary-org slug
(`github.org`), `POST /api/install/app` writes the creation-time quartet
(`github.app_id`, `github.app_slug`, `github.app_client_id`,
`github.app_webhook_secret`),
`POST /api/install/credentials` writes the generated pair
(`github.app_private_key`, `github.app_client_secret`), and
`POST /api/install/registry` writes the ghcr token — each action requires
all of its keys non-empty, writes all-or-none, and **suffix-resets every later
step in the same transaction** (§Redo and suffix invalidation). All are
**ungated**: no session can exist before the credentials enable login, the
same accepted posture as the ungated first-install migrations button. There is
no generic settings REST surface — every settings write goes through a
purpose-built action, and reads go through the model accessors.

The claim: the GitHub App Setup URL is a browser redirect, so it lands on the
**webui install page** (a GET that only renders, carrying GitHub's
`installation_id` query param); the page then submits `POST /api/install/claim`
with that id (session-gated: resolve installation→org via the App API, verify
the session user is an org owner, write the values) — the write sits behind a
POST, never the landing GET. The session requirement is why the auth fragment
mounts pre-install. A browser that lost the id (a fresh tab, a cleared session)
re-fires the redirect by re-saving the installed App's repository selection on
GitHub — the claim panel says so and links the installation page. The write
needs the `settings` table, so the claim runs **after** migrations, which is
why `claimed` is the last state entry:

| Key                            | Value                          |
|--------------------------------|--------------------------------|
| `install.org_id`               | bigint — the rename-stable key |
| `install.org_login`            | current org login              |
| `install.installation_id`      | the GitHub App installation id |
| `install.installed_by_user_id` | the seed admin                 |
| `install.installed_by_login`   | seed admin's login at install  |
| `install.installed_at`         | timestamp (RFC 3339)           |

"Installed" means **every `install.*` value is non-empty**; an absent key reads
as empty, the not-yet-claimed state, and the claim writes all keys or none.
Re-org = clear the `install.*` values + re-install (the App credentials survive
unless the App itself changes).

## App creation — by hand, guided by the install page

The GitHub App is **created by hand** on GitHub; there is no manifest
auto-exchange flow. The **webui install page is the canonical, sole operative
home** for the creation steps — it renders the running server's live URLs at
install time, so the operator copies real values rather than guessing them. The
content splits across the two App wizard steps as GitHub's flow does — with the
`org` step ahead of both:

**`server` — name the server**: one field, the server's public URL, saved as
`server.public_url`. The panel pre-fills from the browser origin when the
setting is empty; every later panel's server-side URL (Homepage, callback,
webhook, Setup URL) renders from the saved value, and any page whose browser
origin differs from it shows the mismatch warning.

**`org` — name the primary org**: one field, the org's slug, saved as
`github.org`. The instructions say what it is for — every GitHub link the later
steps render is built from it — and that a wrong slug simply 404s those links.
Before this step is saved, later panels have no slug to build with and show the
literal placeholder forms of their URLs.

**`app-created` — the creation form**, top to bottom as GitHub lays it out:

- the Homepage URL (the saved `server.public_url` — GitHub requires one, and the
  server's public URL is the honest value);
- the OAuth callback and webhook URL (built from `server.public_url` — callbacks
  and hooks target the backend directly, never the webui);
- the **Setup URL** — `<public URL>/install/`, with **Redirect on update** checked.
  GitHub redirects here with `installation_id` after every install, which is
  what makes the first-time install land back on the wizard unassisted — set at
  creation time, before any install can happen;
- the webhook secret, **minted by the wizard**: the field arrives pre-filled
  with a generated value (with a regenerate control), and the operator copies it
  into GitHub's form — the wizard saves the value GitHub was given;
- exact permissions, named as GitHub's form groups them — *Repository*: Contents
  (Read and write), Metadata (Read-only); *Organization*: Members (Read-only — the
  claim reads org memberships to prove ownership);
- the event subscription: **Push, and only Push** — GitHub delivers only
  subscribed events, and push is the one event srv consumes (the tag-watch that
  queues builds); an App with no subscription delivers nothing and tag pushes
  never build;
- restrict-to-managed-org;
- the entry form for what creation yields: App id, the App's URL — the wizard
  extracts the slug from either page form (`…/settings/apps/<slug>` or
  `github.com/apps/<slug>`; a bare slug is accepted too, and only the slug is
  what saves) — client id, and the webhook secret as given.

**`app-credentials` — the generated keys**, on the created App's settings page —
linked directly (`github.com/organizations/<org>/settings/apps/<slug>`) — in the
order GitHub's page presents them (client secrets sit above private keys):

- generate a client secret, entered in a text field;
- generate a private key — GitHub delivers it as a `.pem` **download**, so the
  form takes a **file picker**, reads the file's text in the browser, and
  submits it as the PEM value (the wire contract is unchanged: text in JSON);
- the pair saves as their `github.app_*` settings.

**`registry-token` — the ghcr push credential**, created on GitHub by hand like
the App:

- why a PAT: the server pushes images to ghcr, and ghcr accepts no App-derived
  credential ([vendor/ghcr-auth.md](../vendor/ghcr-auth.md));
- a direct link to the classic-PAT creation page, naming the one scope:
  `write:packages` — nothing else;
- the note that the token acts for whoever creates it: prefer a machine user or
  an org owner;
- the entry form for the token, saved as `registry.ghcr.io.token`.

**`app-installed` — install the App**: a direct link to the App's install
page — `github.com/organizations/<org>/settings/apps/<slug>/installations` —
opened in a **new tab** (the step leaves the platform site; the wizard tab
stays put and states why: GitHub's Setup URL redirect returns to the wizard on
its own). The operator installs the App on the managed org there; GitHub
redirects back to `<origin>/install/?installation_id=…`, the check sees the
installation via the App API, and the step is done. No Setup URL work happens
here — it was set on the creation form.

**`claimed` — the org-owner claim**: sign in (session state, not step work —
the panel offers it when no session exists), then one action: Claim, which
binds the installation to the server (§The install settings) and completes
the wizard — the server then restarts itself into the product composition
(§Boot composition).

A `docs/guides/` conceptual how-to is **deferred** — a thin pre-deploy discovery
doc, added later only if the need proves real, never a second maintained copy of
the steps.

## The org-wide GitHub→Flux delivery webhook

Delivery is triggered by GitHub's `registry_package` webhook firing the cluster's
Flux `Receiver` (the **GitHub→Flux** axis — see
[config-allocation.md](config-allocation.md) for the flow and
[scaffolding.md](scaffolding.md) for the one-per-cluster `Receiver`). This webhook
is **org-wide, wired once, never minted per repo** — cluster-side delivery
plumbing, wired by hand at cluster bring-up, **not part of the install wizard**:
the wizard's App webhook already carries every event srv needs from all installed
repos, and this one exists only because `registry_package` targets the Receiver,
not srv. Server-automated wiring is **deferred to the flux-integration slice** —
which will also exercise adding a wizard step to an already-installed server. The
old per-repo `POST /api/repos/{owner}/{repo}/flux-webhook` endpoint is **VOID**
and the rebuild drops it.

## Migrations — never at boot

Migrations **never auto-run at boot**. Two paths reach the same schema:

- **CLI** — `./platform srv data migrate`, run before a deploy so the new boot
  comes up already migrated.
- **Installer button** — the `migrations` remediation on the install page, which `POST`s
  `/api/install/migrations` (an installer-fragment action). Installer actions —
  migrations and credentials alike — are **deliberately ungated**: the deployment
  URL is treated as secret until the install record exists, and the boot
  install-route switch removes the exposure once the server is completely
  installed. No session requirement applies.

Because a pending migration drops the **whole product to the installer** (intended
— the product API refuses to mount against an out-of-date schema), the CLI pre-run
is the standard mitigation: migrate first, then deploy, and the new process boots
straight into the product.
