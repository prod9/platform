# Platform server authenticates as a GitHub App; zero platform-side RBAC

Date: 2026-06-29
Status: **revised** — 2026-07-18 addendum below; the original ruling stands unchanged.

Frozen *why*; current state lives in [platform-server.md](../spec/platform-server.md)
(intended/target — the `srv/` layer is not yet built).

## The ruling

The platform server (`srv`) authenticates as a **GitHub App** and holds **zero
platform-side RBAC** — no permission tables, no roles. Authorization is delegated entirely
to GitHub:

- A user who can access the repo can trigger its builds.
- **Deploy authority = git push permission on the infra repo.**

## Why zero RBAC

A deploy **is** a commit to the infra repo — the committed image-literal model, where the
operator hand-edits the image ref and platform never rewrites their CUE (see
[render-is-pure-function-of-committed-git](2026-06-26-render-is-pure-function-of-committed-git.md)).
So GitHub's write bit on the infra repo *is* the deploy gate, with nothing to configure.
Storing a permission table would duplicate — and inevitably drift from — the authorization
GitHub already holds. The consequence for credentials: platform acts with the **triggering
user's GitHub identity** where attribution/gating matters, never a single god credential
that would force platform to decide who-can-do-what.

## Why a GitHub App (not an OAuth App)

The GitHub App is the GitHub-sanctioned integration model (the path GitHub Actions,
Vercel, Jenkins, post-migration CircleCI, and Buildkite's control plane take). It removes
the two failure modes an OAuth-token approach forces you to work around:

- a stored long-lived **per-user secret**, and
- a **bus-factor** on whoever connected the repo.

The server owns the App and creates it once at server setup via GitHub's App Manifest flow
— no manual private-key copy. Two token types split by need: an **installation token**
(`platform[bot]` identity, for autonomous webhook-driven build/publish) and a
**user-to-server token** (acts as the user, bounded by the user's access ∩ the app's
granted permissions, for a deploy where attribution + per-user gating matter). The user
token restores implicit authz — it cannot exceed the user's reach — so the explicit access
check is only needed on the installation-token path.

## Rejected

- **A platform permission table / roles.** Duplicates GitHub's authorization and drifts
  from it. The infra-repo push bit is already the deploy gate. Rejected.
- **An OAuth App with a stored per-user token.** Imports a long-lived per-user secret and
  a bus-factor on the connector — the two failure modes the App model removes. Rejected.
- **A single god credential.** Forces platform to decide who-can-do-what, reintroducing
  the RBAC this ruling eliminates. Rejected.

## Accepted trade

Either token only reaches a repo where the App is **installed** (and, for the user token,
where the user also has access). Unlike a raw OAuth token, a GitHub App user token cannot
reach every repo the user can — the install is the gate, and is also what enables
webhooks. Accepted.

## 2026-07-18 addendum — stress-tested against observability; holds

The zero-RBAC model was pressure-tested against the one action that looked most likely to
force a permission table: **cluster / Flux delivery observability** (letting a user see a
repo's reconcile state). It **holds, unchanged**, with two enablers:

- **Authz stays GitHub-derived.** A user's right to *view* a repo's delivery state is the
  same **infra-repo rights check** already used for deploy; the read itself is performed
  by the **pod ServiceAccount** reading Flux CR state (`OCIRepository`/`Kustomization`).
  No platform role, no stored permission.
- **repo→namespace mapping is routing, not authz.** It is *not* derivable from the repo
  name (`bluepages-infra` → `haachang.com/s9-haachang`), so it is **discovered from
  existing cluster metadata** (`Kustomization`→`sourceRef`→`OCIRepository`→image) and
  cached in a **fat session** (which also holds the rights-derived repo list, hence a
  TTL). It is never a stored mapping table.

Consequences that follow: repo-first information architecture (no namespace leak), **no
baseline change**, and **no informer** (over-engineered — smart session caching suffices
if ever needed). The app→infra handoff stays manual. Full worked derivation (points 1–17):
[`2026-07-18-srv-rbac-observability.md`](../scratch/2026-07-18-srv-rbac-observability.md).
The observability *endpoint + UI surface* is a separate, still-open design question — this
addendum rules only that zero-RBAC survives it.
