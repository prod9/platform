# Installation

Status: **target design — not yet built.** The `srv/` skeleton predates this
model; the installer fragment, the `GET /api/install` state surface, and the
boot-composition gating described here are the intended design the rebuild
implements. The auth model this sits on is frozen in
[platform-server-github-app-zero-rbac](../decisions/2026-06-29-platform-server-github-app-zero-rbac.md);
the route surface lives in [platform-server.md](platform-server.md).

## What installation is

A platform server governs **one GitHub org**. Installation is the one-time act
of pointing a fresh server at that org: creating the GitHub App, installing it,
wiring the org-wide delivery webhook, and running migrations. Until all of that
is true, the server is **not completely installed** and serves only the installer.

The whole concern lives in a single **installer fragment** (an fx app fragment).
Product fragments — session, users, auth callbacks, hooks, builds — have **zero
install awareness**: they are mounted only once the server is completely installed.

## The `GET /api/install` state surface

The installer exposes one read endpoint, `GET /api/install`, returning an
**ordered list of state entries**. Each entry is `done`, `pending`, or `error`.
The **first non-`done` entry is the `next` step; the webui picks the component it
renders from that entry.

| Entry             | Check                                       | Not-done meaning                           |
|-------------------|---------------------------------------------|--------------------------------------------|
| `db-reachable`    | `SELECT 1;`                                 | `error` → "Database connection problem: …" |
| `app-credentials` | App id / private key / secrets in fx config | `error` when missing                       |
| `app-installed`   | the App is installed on the bound org       | `pending` → install button                 |
| `migrations`      | schema is current                           | `pending` → run button; dirty → `error`    |

Remediations are **convergent and re-runnable**. A dirty migration and a DB error
surface as **errors**, not action buttons — they are operator conditions, not
one-click fixes.

"Completely installed" is the conjunction of all four: `db-reachable ∧
app-credentials ∧ app-installed ∧ migrations-current`.

## Boot composition — the installer gates the product API

Boot decides the API composition **once**, from `install.GetState()`:

- **Webui `GET /*` is mounted unconditionally** in both states. It never needs
  remounting.
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

## The install record

A **singleton** row, written by the installer fragment:

| Field                  | Note                           |
|------------------------|--------------------------------|
| `org_id`               | bigint — the rename-stable key |
| `org_login`            | current org login              |
| `installation_id`      | the GitHub App installation id |
| `installed_by_user_id` | the seed admin                 |
| `installed_by_login`   | seed admin's login at install  |
| `installed_at`         | timestamp                      |

App credentials are **not** in the record — they live in fx config (see
[platform-server.md](platform-server.md), "Auth mechanism"). Re-org = delete the
row + re-install.

## App creation — by hand, guided by the install page

The GitHub App is **created by hand** on GitHub; there is no manifest
auto-exchange flow. The **webui install page is the canonical, sole operative
home** for the creation steps — it renders the running server's live URLs at
install time, so the operator copies real values rather than guessing them. Steps
content:

- exact permissions: `contents: write`, `metadata: read`;
- the webhook URL and OAuth callback (the srv backend's own URLs — callbacks and
  hooks target the backend directly, never the webui);
- restrict-to-managed-org;
- the credential→config mapping (the created App's id, private key, and secrets
  go into fx config).

A `docs/guides/` conceptual how-to is **deferred** — a thin pre-deploy discovery
doc, added later only if the need proves real, never a second maintained copy of
the steps.

## The org-wide GitHub→Flux delivery webhook

Delivery is triggered by GitHub's `registry_package` webhook firing the cluster's
Flux `Receiver` (the **GitHub→Flux** axis — see
[config-allocation.md](config-allocation.md) for the flow and
[scaffolding.md](scaffolding.md) for the one-per-cluster `Receiver`). This webhook
is **org-wide, wired once, never minted per repo** — provisioning it is a **manual
step on the webui install page**, alongside the App-creation steps (same operative
home). The old per-repo `POST /api/repos/{owner}/{repo}/flux-webhook` endpoint is
**VOID** and the rebuild drops it.

## Migrations — never at boot

Migrations **never auto-run at boot**. Two paths reach the same schema:

- **CLI** — `./platform srv data migrate`, run before a deploy so the new boot
  comes up already migrated.
- **Installer button** — the `migrations` remediation on the install page.

Because a pending migration drops the **whole product to the installer** (intended
— the product API refuses to mount against an out-of-date schema), the CLI pre-run
is the standard mitigation: migrate first, then deploy, and the new process boots
straight into the product.
