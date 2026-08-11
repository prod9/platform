<!-- derived from: cmd/go/internal/vcs/vcs.go (go master) @ 2026-08-11 -->

# Go vanity imports — how the toolchain resolves `go-import` meta tags

The facts the SPA-baked meta tag relies on, verified against the toolchain's own
resolver (`cmd/go/internal/vcs/vcs.go`):

- **A longer path may declare a shorter prefix.** When a page at
  `https://host/some/pkg?go-get=1` serves a `go-import` whose prefix is a parent
  (e.g. `host` alone), the toolchain re-fetches `https://host/<prefix>?go-get=1`
  and requires that page to declare the same import — `metaImportsForPrefix`. So
  one static tag served on every page of the host works for every package path
  under it.
- **Non-200 bodies are parsed.** The resolver reads meta tags out of error
  responses too; an HTTP status error is reported only when *zero* `go-import`
  tags parse from the body. A 404 fallback page carrying the tag therefore
  resolves normally.
- The toolchain always appends `?go-get=1` to the page it fetches; a static page
  may ignore it.

Tag shape (one per module root):

```html
<meta name="go-import" content="<module-path> git <repo-url>">
```
