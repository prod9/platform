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
| List installations          | `GET /app/installations`                             | JWT                | 200 | array of the same installation objects; paginated (`per_page` max 100, default 30) |
| List installation repos     | `GET /installation/repositories`                     | installation token | 200 | `total_count`, `repositories[]`; `per_page` ≤ 100    |
| Org membership              | `GET /orgs/{org}/memberships/{username}`             | installation token | 200 | `role`: `admin`\|`member`\|`billing_manager`, `state`: `active`\|`pending` |
| Resolve ref → sha           | `GET /repos/{owner}/{repo}/commits/{ref}`            | installation token | 200 | top-level `sha`; `ref` = SHA, `heads/BRANCH`, `tags/TAG` |
| Get the authenticated App   | `GET /app`                                           | JWT                | 200 | `permissions`: map of slug → `read`\|`write`         |
| Get org installation        | `GET /orgs/{org}/installation`                       | JWT                | 200 | one installation object (`id`, `account`, `app_id`)  |
| List org webhooks           | `GET /orgs/{org}/hooks`                              | installation token | 200 | `[]` of `id`, `config` (`url`), `events`, `active`   |
| Create org webhook          | `POST /orgs/{org}/hooks`                             | installation token | 201 | `name` must be `"web"`; `config.url` + `content_type`; `events`, `active` |

## Pagination — the `Link` response header

- Paginated endpoints answer a `Link` header of `<url>; rel="…"` entries; the rels are
  `prev`, `next`, `first`, `last`, and only a subset may be present.
- **The last page is the response with no `rel="next"` entry** — that absence is the
  end-of-list signal, not a short page.
- **Follow the header's URLs verbatim; never construct page URLs by hand** — endpoints
  differ in which query parameters drive paging (`page`, `before`/`after`, `since`),
  so the header URL is the only portable cursor.

## Notes

- **Org owner** = membership with `role: admin` and `state: active`. A 404 from the
  membership endpoint means "not a member". The App needs the **Organization members:
  read** permission for its installation token to read memberships.
- `commits/{ref}` supports the `application/vnd.github.sha` media type — the response
  body is then the bare SHA-1, no JSON. A ref the repo does not have answers 422 (or
  404 when the repo itself is absent) — both mean the caller's ref, not a server fault.
- Installation tokens live ~1 hour; mint per operation, never store.
- `GET /app` permission slugs the wizard compares: `contents`, `metadata` (repository);
  `members`, `organization_hooks` (organization). Values are only ever `read` or
  `write`; `write` implies `read`.
- Org webhook writes need the App's **Organization webhooks: read and write**
  permission; `registry_package` is the event a GHCR publish delivers.
- `GET /orgs/{org}/installation` documents only 200; an org the App is not installed
  on answers GitHub's standard 404 (undocumented on this endpoint — verified against
  the endpoint page 2026-08-11, which lists no error codes).
