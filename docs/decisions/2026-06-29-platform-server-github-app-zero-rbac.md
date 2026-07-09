# Platform server authenticates as a GitHub App; zero platform-side RBAC

Date: 2026-06-29
Status: **accepted**

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
