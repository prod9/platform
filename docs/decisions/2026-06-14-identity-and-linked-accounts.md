# ADR: Identity model — internal users + linked accounts

- **Status:** Accepted
- **Date:** 2026-06-14
- **From:** 2026-06 platformv2 design walk

## Context

The server exists largely for identity/RBAC. We need GitHub login now, but custom IdPs and
linked *services* (e.g. Sentry) soon — without reworking the schema.

## Decision

Internal **`users`** are the anchor. **`identities`** rows link external accounts:
`(user_id, provider, provider_id, metadata jsonb, kind)` with `unique(provider,
provider_id)`. Rules:

- `provider_id` is the provider's **immutable id**, not the username (renames don't break
  links; username lives in `metadata`).
- `kind` separates `login` providers from `service` links; the adapter declares capability.
- Tokens in `metadata` are encrypted at rest (same key as secrets).
- Auth providers sit behind an `IdentityProvider` interface, never hardcoded to GitHub.
  *(The original "claim→role mapping" authz clause is superseded: platform holds zero
  RBAC — see
  [platform-server-github-app-zero-rbac](2026-06-29-platform-server-github-app-zero-rbac.md).)*
- Platform issues its own session token; downstream consumes platform identity.
- Same **verified** email across **trusted** providers auto-links to one user; a per-provider
  `trust` + `email_verified` flag gates it.

## Alternatives rejected

- **Keying everything on the GitHub user/username** — brittle on rename, no path to other
  providers.
- **No email auto-link** — too strict; the gated verified+trusted form is safe enough.
- **Email auto-merge ungated** — account-takeover footgun via unverified-email providers.

## Consequences

- v2 ships only the GitHub adapter; Google/Sentry/custom slot in with zero schema change.
- A single human can carry many provider identities and service links under one user.
