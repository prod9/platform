# Platform never reaches into the container

**2026-07-27**

## Ruling

The inside of a built container is the application's own concern. Platform decides how an
image is *assembled*; it does not configure what runs inside one. A port is the canonical
case: platform never reads a module's `port` into the image, never declares an exposed port
on the app's behalf, and never templates a listen address into an app's config.

An application that needs a configurable port reads `PORT` itself, the way 12-factor
prescribes. Where platform supplies the server — `pnpm/static`, whose Caddy is platform's
own — Caddy is that application: its config listens on `:{$PORT:3000}`, reading the variable
from its own environment at start and falling back to 3000. Platform writes that reference
and never the answer. Nothing sets `PORT` on the container's behalf — not the module's
`port` key, not `preview`, not a framework — so the value can only arrive from outside, from
whoever runs the image.

`preview` forwards the port the operator configured and makes no container change of any
kind. The `port` key exists for that forwarding and nothing else.

## Why

A build tool that configures the app's interior has to know the app's interior, and every
framework then grows a private channel for reaching in — an env var here, a config splice
there. Those channels are invisible from the manifest, untestable from outside, and each one
is a place where platform's idea of the app and the app's own idea can silently disagree.

The narrower line — platform assembles, the app configures itself — needs no such channel.
It also keeps every framework's config a constant, which is what makes the config auditable
by reading one file rather than by building an image and looking inside.

## Consequences

- `framework/caddy.go` is a constant. Nothing in the Caddyfile derives from the project being
  built: not the served root, not a path matcher, and not the listen port — `{$PORT:3000}` is
  the same bytes in every static image, resolved by Caddy at start and never by platform.
- No framework declares `WithExposedPort`. Kubernetes ignores the OCI field anyway, and
  declaring it would state a port platform does not own.
- A framework may not special-case a bundler's output layout. `pnpm/static` is handed one
  output directory and knows nothing else about the project — so no `/_astro/*` rule, and no
  immutable-cache tier that would need one. Guessing wrong there serves a stale asset for a
  year.
- The surface is spec'd in [`frameworks.md`](../spec/frameworks.md#the-static-familys-http-surface).

## Scope

This governs platform's relationship with the *application's* runtime configuration. It says
nothing about the image's construction — base image, package sets, cache mounts, the FHS
tree, the runner's default args — all of which are platform's to decide and are spec'd
elsewhere.
