<!-- derived from: docs.github.com "Working with the Container registry" +
github.com/orgs/community/discussions/171423 + /26921 @ 2026-08-05 -->

# ghcr.io authentication

What credentials GitHub's container registry actually accepts, for `publish` and for
pulling rocks with the `REGISTRY_USERNAME`/`REGISTRY_PASSWORD` credentials
([Lua render](../spec/lua-render.md#rocks)).

## What works

| Credential                        | docker login / push               |
|-----------------------------------|-----------------------------------|
| **PAT (classic)**, `write:packages` | ✅ the only supported credential outside Actions |
| Actions `GITHUB_TOKEN`            | ✅ but exists only inside a workflow run |
| Fine-grained PAT                  | ❌ packages not in its permission model |
| GitHub App **installation token** | ❌ login "succeeds", push/pull denied |
| OAuth app token w/ `write:packages` | ❌ "token does not match expected scopes" |
| GitHub App user-access token      | ❌ same refusal class               |

GitHub staff, on installation tokens (discussion 171423, still open as of 2026-07):
"GHCR does not yet accept GitHub App installation tokens for authentication." On OAuth
tokens (discussion 26921): "GHCR can't accept App tokens, only PATs for now."

Consequences for platform:

- Registry credentials are an operator-supplied classic PAT; no GitHub App credential
  can stand in for one.
- Reading (`docker pull`) needs `read:packages`; `write:packages` implies it.

## Basic-auth username — unverified

`docker login ghcr.io -u USER` with a PAT: GitHub's docs show the PAT owner's login as
`USER`; whether ghcr actually validates the username against the PAT is **unverified**
(commonly reported as ignored, no authoritative source found). If a push is denied with a
token that is known good, the username/PAT-owner mismatch is the first suspect — verify
by logging in with the PAT owner's real login.
