# The go-import page is served by Cloudflare; platform publishes no image of itself

- **Date:** 2026-11-25
- **PR:** manual
- **Status:** accepted

Current contract: [scaffolding](../spec/scaffolding.md), "Installing".

## Decision

`platform.prodigy9.co` answers Go's `?go-get=1` lookup from a Cloudflare Worker managed
by OpenTofu in `infra-tf`, not from platform. platform v1.0 has no command that serves
the page and builds no image of itself: its own `platform.toml` declares no module.

chakrit, 2026-11-25, after being offered a hidden `vanity` command first ("i mean you
build it, new command.") and then Cloudflare Pages or Workers: "ok do that".

## Rationale

**Something must serve the page.** Every launcher runs `go install
platform.prodigy9.co@<version>`, and Go resolves that vanity path by fetching an HTML page
with a `go-import` meta tag. The repository is private
([repo stays private](2026-06-26-repo-stays-private.md)), so Go's public module proxy
cannot answer instead. Until v1.0 the platform server served the page; the server is
removed ([server removed](2026-11-25-server-removed-platform-is-a-cli.md)).

**Why not DNS or a redirect.** DNS cannot serve a page. A redirect to GitHub does not
help: the tag's import-prefix must be a prefix of the path being fetched
(`go help importpath`), and GitHub's tag names `github.com/prod9/platform`, not
`platform.prodigy9.co`.

**Why not a `vanity` command in platform.** It would keep a server, an image and a cluster
deployment alive for one static answer. The hostname is already a proxied Cloudflare
record (`infra-tf/prodigy9-co.tf`), so a Worker serves the same answer with nothing to
build or run on the cluster.
