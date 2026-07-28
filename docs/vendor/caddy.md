<!-- derived from: https://caddyserver.com/docs/caddyfile/{concepts,directives/{encode,file_server}} +
github.com/caddyserver/caddy v2.11.4 modules/caddyhttp/fileserver/staticfiles.go @ 2026-07-28 -->

# Caddy (the static family's webserver)

`pnpm/static` runners serve their bundle with Caddy, installed from Wolfi as `caddy`
(2.11.4-r0 — see [`wolfi.md`](wolfi.md) for what that package lays down). Our side of the
contract — which directives we set and why — is
[`../spec/frameworks.md`](../spec/frameworks.md), "The static family's HTTP surface"; this
file is only for looking *their* behavior up.

## Content-Type comes from Go's MIME table, and only from there

`file_server` has no MIME map of its own. It calls Go's `mime.TypeByExtension` on the
file's extension, and when that returns nothing it sends **no `Content-Type` header at
all** rather than letting Go sniff the body (`staticfiles.go`, v2.11.4):

```go
mtyp := mime.TypeByExtension(filepath.Ext(filename))
if mtyp == "" {
	// do not allow Go to sniff the content-type
	respHeader["Content-Type"] = nil
} else {
	respHeader.Set("Content-Type", mtyp)
}
```

Two consequences we depend on:

- **The system MIME table is the whole story.** Go's `mime` package reads
  `/etc/mime.types`, which on Wolfi arrives with `mailcap` — that is why `withRunnerPkgs`
  installs it, and why a font or a `.webmanifest` gets the right type for free.
- **A type missing from that table cannot be fixed by a `file_server` option.** The only
  route is a per-path `header` rule in the Caddyfile.

## `encode` defaults

| Item             | Default                                                                                                         |
|------------------|-----------------------------------------------------------------------------------------------------------------|
| Encoders         | `zstd` (preferred) then `gzip` when the directive names none                                                    |
| `minimum_length` | 512 bytes — smaller responses are sent uncompressed                                                             |
| Matched types    | a built-in list: `text/*`, `application/json*`, `application/javascript*`, `font/*`, `image/svg+xml*`, and more |

The match list is Caddy's own and needs no configuration for a static site; we name only
the encoders.

## `handle_errors` and the response status

`handle_errors` defines routes invoked when a handler errors (404, 403, …). Rewriting to a
status-named page and re-running `file_server` serves that page **at the original error
status** — the code is not reset to 200 by the rewrite. A `status` subdirective exists to
override it deliberately; we do not use one.

## Environment variables in the Caddyfile

`{$NAME}` expands to the environment variable's value, and `{$NAME:default}` supplies a
default for when the variable is unset. The expansion happens **before Caddyfile parsing
begins** — it is textual, so a variable can expand to an empty value, a partial token, or
several tokens and lines. Being pre-parse, it reads the environment of the `caddy run`
process at config load, which for our runner is container start.

This is the one substitution allowed in a site address: Caddy's docs state that
placeholders cannot be used in addresses, but Caddyfile environment variables can. The
runtime `{env.NAME}` placeholder is the other form — resolved per request, unsupported in
addresses, and not something we use.

## Not verified here

Anything not listed above. Caddy's docs are the source of truth
(<https://caddyserver.com/docs/>); re-read them rather than extending this file from
recall.
