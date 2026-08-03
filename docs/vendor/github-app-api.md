<!-- derived from: docs.github.com REST API 2022-11-28 @ 2026-08-03 -->

# GitHub App API — the endpoints `srv/github` drives

## App JWT

- Algorithm **RS256**, signed with the App's PEM private key.
- Claims: `iss` = the App's **client ID** (recommended over the numeric app ID; GitHub
  accepts both), `iat` = now − 60s (clock-drift allowance), `exp` ≤ 10 minutes out.
- Sent as `Authorization: Bearer <jwt>` — a JWT must use `Bearer`, never `token`.

## Endpoints

| Call                        | Method+path                                          | Auth               | OK  | Response                                             |
|-----------------------------|------------------------------------------------------|--------------------|-----|------------------------------------------------------|
| Mint installation token     | `POST /app/installations/{id}/access_tokens`         | JWT                | 201 | `token`, `expires_at`, `permissions`                 |
| Get installation            | `GET /app/installations/{id}`                        | JWT                | 200 | `id`, `account` (`login`, `type`), `app_id`          |
| List installation repos     | `GET /installation/repositories`                     | installation token | 200 | `total_count`, `repositories[]`; `per_page` ≤ 100    |
| Org membership              | `GET /orgs/{org}/memberships/{username}`             | installation token | 200 | `role`: `admin`\|`member`\|`billing_manager`, `state`: `active`\|`pending` |
| Resolve ref → sha           | `GET /repos/{owner}/{repo}/commits/{ref}`            | installation token | 200 | top-level `sha`; `ref` = SHA, `heads/BRANCH`, `tags/TAG` |

## Notes

- **Org owner** = membership with `role: admin` and `state: active`. A 404 from the
  membership endpoint means "not a member". The App needs the **Organization members:
  read** permission for its installation token to read memberships.
- `commits/{ref}` supports the `application/vnd.github.sha` media type — the response
  body is then the bare SHA-1, no JSON.
- Installation tokens live ~1 hour; mint per operation, never store.
