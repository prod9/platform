# www — docs review site

A human-facing presentation of this repo's design record, **synthesized** from `docs/`. It
is derived and downstream: `docs/` is the source of truth, `www/` is a rewrite for
readers.

## Rules

- **Never edit a page here to change meaning.** Edit the source under `docs/`, then
  regenerate the affected page. A page changed in place is a bug.
- **Synthesize, don't mirror.** `docs/` sorts for maintainers (permanence, type); this
  site sorts for readers (topic, journey). Collapse, reorder, and rewrite — do not
  1:1-port markdown files.
- **Carry provenance.** Each page in `pages/` heads with the sources it derives from and
  the commit they were read at: `<!-- derived from: docs/spec/foo.md @ <commit> -->`. When
  a source changes, regenerate the page in the same commit.

## Layout

- `index.html` — shell + reader-journey nav; htmx swaps fragments into `#content`.
- `pages/` — authored HTML fragments, one per synthesized page.
- `assets/style.css` — readability styling.

## Preview

A static server is required — htmx fetches fragments over HTTP, so opening `index.html`
via `file://` fails.

```sh
python3 -m http.server -d www 8000   # then open http://localhost:8000
# or: mongoose       (run inside www/)
# or: npx serve www
```

## Publish

```sh
scripts/docs-site-deploy.sh          # pushes www/ -> gh-pages on remote `gh`
```

One-time: enable GitHub Pages → `gh-pages` branch in repo settings.
