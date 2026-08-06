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
of pointing a fresh server at that org: running migrations, creating the GitHub
App, supplying the registry push token, installing the App, and wiring the
org-wide delivery webhook. Until all of that
is true, the server is **not completely installed** and serves only the installer.

The install flow is a **wizard**: the webui walks the state entries as
step-by-step guided pages, and every value the operator enters — the App
credentials, then the org binding — is saved into the settings app as its step
completes, advancing the wizard to the next step. **All install-time settings
live in settings**; the deployment supplies only `DATABASE_URL` (and listen
address) through fx config/env.

The wizard UI holds four rules:

- **Progress is always visible.** The full ordered state list renders on every
  step, statuses as returned by `GET /api/install`.
- **One step at a time.** Below the progress list sits exactly one step panel —
  the one for the first non-`fully_ready` entry.
- **Each panel is operative.** It carries the detailed instructions the human
  operator follows, direct links to the external pages the step works on (the
  GitHub App creation page, the Apps list for the Setup URL), and — where the step takes values — the input fields and a save
  action posting the step's installer action.
- **Restartable, always converging.** The wizard holds no step state of its
  own: every save's response (and every page load) is a fresh state read, so a
  reload, a failure, or out-of-order work always lands the operator back on
  the first unfinished step, with the progress list telling the truth.

The whole concern lives in a single **installer fragment** (an fx app fragment).
Product fragments — hooks, builds — have **zero install *flow* awareness**: they
are mounted only once the server is completely installed and never ask "am I
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
**ordered list of state entries**. Each step is a check unit (`Step`: a name
and a `Check`) and reports one of five states; the **first non-`fully_ready`
entry is the `next` step; the webui picks the component it renders from that
entry.

| State                   | Meaning                                                        |
|-------------------------|----------------------------------------------------------------|
| `fully_ready`           | the step's condition holds                                     |
| `partially_ready`       | some but not all of the step's work is done                    |
| `not_started`           | none of it yet — the step's action is the next move            |
| `intervention_required` | an operator condition, not a one-click fix (dirty schema, bad `DATABASE_URL`) |
| `""` (unknown)          | the check itself failed — indeterminable, message carries why  |

Unknown is the **zero value** on purpose: an unset state reads as "nobody
knows", never as a verdict. Entries carry `message` alongside the state when
the check produced an error.

| Entry             | Check                                    | Non-ready meaning                          |
|-------------------|------------------------------------------|--------------------------------------------|
| `db-reachable`    | connectivity ping                        | `intervention_required` → connection fix   |
| `migrations`      | schema is current                        | `not_started`/`partially_ready` → run button; dirty → `intervention_required` |
| `app-created`     | the creation-time values — `github.app_id`, `github.app_client_id`, `github.app_webhook_secret` — all have values | `not_started` → the create-the-App wizard step |
| `app-credentials` | every `github.app_*` setting has a value **and the App carries the required permissions** | `not_started` → the generated-keys wizard step; `partially_ready` → App reachable but under-scoped, message names the gap |
| `registry-token`  | the `registry.ghcr.io.token` setting has a value | `not_started` → the registry-PAT wizard step |
| `app-installed`   | every `install.*` setting has a value    | `not_started` → org-owner claim            |

The App steps are two because GitHub's flow is two: **creating** the App yields
its id and client id (and echoes back the webhook secret the operator typed into
the form), while the **private key and client secret are generated afterward** on
the created App's settings page. Each step saves what its GitHub page produced,
so a reload lands the operator exactly where the real-world flow stands.

Two steps can be `partially_ready`: `migrations` (some applied, some pending)
and `app-credentials` (credentials saved, but `GET /app` reports a permission
below the required set — the message names exactly which). The claim writes
all-or-none, so it never is. Remediations are **convergent and re-runnable**.

The credentials check compares `GET /app` (JWT auth) against the required set:
`contents: write`, `metadata: read`, `members: read` — `write` satisfies a
`read` requirement. An App whose credentials fail JWT/API entirely is
`unknown` with the error, not partially ready.

"Completely installed" is the conjunction: **every entry `fully_ready`**. The
order matters: every install-time value lives in settings, and the settings
table exists only once migrations ran — so **migrations precede every
settings-backed entry**; `app-created` precedes `app-credentials` (the keys are
generated on the App that creation produced), `registry-token` follows the App
steps (it is the last operator-typed value), and `app-installed` stays last
(the claim needs credentials to talk to GitHub).

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

Boot decides the API composition **once**, from `install.GetState()`:

- **Webui `GET /*` and the auth fragment are mounted unconditionally** in both
  states. They never need remounting.
- **Not completely installed** → installer *action* endpoints are mounted;
  product `/api/*` is **not**. `GET /api/install` is served here — it is **part
  of the gated installer fragment, not an always-available endpoint**.
- **Completely installed** → product `/api/*` is mounted; the installer actions
  are gone.

The **installer→product transition is a process restart** — boot decides
composition, there is no in-process hot-swap.

### The SvelteKit SPA drives the installer-vs-app view

The redirect to the installer is **SPA code, not the backend**. The root-layout
guard probes `GET /api/install`:

- **200 + not-installed** → the SPA redirects to `/install` and runs the flow.
- **404** (installer fragment absent) → completely installed → render the app.

`GET /api/install` is deliberately **not always-available** — its presence *is*
the signal. Depending on 404-as-signal is accepted for now.

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
| `github.app_private_key`    | PEM private key (PKCS1 or PKCS8)   |
| `github.app_webhook_secret` | webhook HMAC secret                |
| `github.app_client_id`      | user-OAuth client id               |
| `github.app_client_secret`  | user-OAuth client secret           |

`srv/github`'s `LoadApp` reads these settings — **App credentials live in the
database, not fx config**; srv and the worker read the same rows
([platform-server.md](platform-server.md), "Auth mechanism"). Secret values in
Postgres is the allocation config-allocation.md already assigns the server;
encryption at rest remains open there.

**The registry token** rides the same store, keyed by registry host:

| Key                        | Value                                        |
|----------------------------|----------------------------------------------|
| `registry.<host>.token`    | classic PAT with `write:packages`, per registry |

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

The wizard's typed steps post three installer-fragment actions, one per page
the operator works: `POST /api/install/app` writes the creation-time trio
(`github.app_id`, `github.app_client_id`, `github.app_webhook_secret`),
`POST /api/install/credentials` writes the generated pair
(`github.app_private_key`, `github.app_client_secret`), and
`POST /api/install/registry` writes the ghcr token — each action requires
all of its keys and writes all-or-none. All are **ungated**: no session can
exist before the credentials enable login, the same accepted posture as the
ungated first-install migrations button. There is no generic settings REST
surface — every settings write goes through a purpose-built action, and reads
go through the model accessors.

The claim: the GitHub App Setup URL is a browser redirect, so it lands on the
**webui install page** (a GET that only renders, carrying GitHub's
`installation_id` query param); the page then submits `POST /api/install/claim`
with that id (session-gated: resolve installation→org via the App API, verify
the session user is an org owner, write the values) — the write sits behind a
POST, never the landing GET. The session requirement is why the auth fragment
mounts pre-install. The write needs the `settings` table, so the claim runs
**after** migrations, which is why `app-installed` is the last state entry:

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
content splits across the two App wizard steps as GitHub's flow does:

**`app-created` — the creation form**, top to bottom as GitHub lays it out:

- the Homepage URL (the server's own origin — GitHub requires one, and the
  running host is the honest value);
- the OAuth callback and webhook URL (the srv backend's own URLs — callbacks and
  hooks target the backend directly, never the webui);
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
- the entry form for what creation yields: App id, client id, and the webhook
  secret as given.

**`app-credentials` — the generated keys**, on the created App's settings page:

- generate a private key (a PEM download) and a client secret;
- the entry form for the pair, saved as their `github.app_*` settings.

**`registry-token` — the ghcr push credential**, created on GitHub by hand like
the App:

- why a PAT: the server pushes images to ghcr, and ghcr accepts no App-derived
  credential ([vendor/ghcr-auth.md](../vendor/ghcr-auth.md));
- a direct link to the classic-PAT creation page, naming the one scope:
  `write:packages` — nothing else;
- the note that the token acts for whoever creates it: prefer a machine user or
  an org owner;
- the entry form for the token, saved as `registry.ghcr.io.token`.

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
