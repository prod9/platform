<!-- derived from: docs.github.com "Working with the Container registry" +
github.com/orgs/community/discussions/171423 + /26921 @ 2026-08-05 -->

# ghcr.io authentication

What credentials GitHub's container registry actually accepts — the facts that ruled
out every App-derived credential for the server's publish path.

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

- The server's publish path cannot mint its registry credential from the installed
  App the way the clone path does (`install.Token`) — an operator-supplied classic
  PAT is the only shape that works, hence the wizard's registry-token step.
- Reading (`docker pull`) needs `read:packages`; `write:packages` implies it.

## Basic-auth username — unverified

`docker login ghcr.io -u USER` with a PAT: GitHub's docs show the PAT owner's login as
`USER`; whether ghcr actually validates the username against the PAT is **unverified**
(commonly reported as ignored, no authoritative source found). The publish path sends
`install.installed_by_login`. If a push is denied with a token that is known good, the
username/PAT-owner mismatch is the first suspect — verify by logging in with the PAT
owner's real login.
