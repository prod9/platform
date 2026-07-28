# Cutting a release

Takes a green `main` to a published GitHub release: a notes file, a tag, a pushed branch, a
re-recorded golden, and a release page carrying the notes.

The notes are drafted **outside the repo**, in `/tmp`. Their permanent home is the GitHub
release page, so a copy in the tree is a second original that drifts, and `platform release`
refuses a dirty worktree anyway — an uncommitted draft would block the tag it exists to
describe. Nothing here commits until step 5, and that commit is the tag's own drift.

## 0. Preconditions

```sh
git status --short          # must be empty
go test ./...               # must pass
./test.sh                   # must report UNCHANGED
```

A `CHANGED` golden here is unreviewed drift from earlier work — settle it before releasing,
never in the same diff as the release. Once the tag exists, the *only* line smoke may move
is the launcher's version pin (step 5), and that rule only holds if the golden was clean
going in.

## 1. Compile the changelog

```sh
git fetch gh --tags
git describe --tags --abbrev=0                       # the previous tag, e.g. v0.9.16
git log --reverse --format='- `%h` %s' v0.9.16..HEAD > /tmp/v0.9.17-release.md
```

🚨 **Capture to a file and read the file.** `git log` on a release-sized range is long
enough that lowfat trims it, and a trimmed list read as complete under-reports the release
by an order of magnitude — the last line says how many were dropped, and only that line
says so.

## 2. Write the notes file

One file, `/tmp/<version>-release.md`, two sections:

- **Changelog** — the step-1 list, verbatim: one line per commit, hash and subject. No
  editing, no grouping, no judgement. It is the record of what went in.
- **Release notes** — **user-side impact only.** What someone running platform, or running
  an image platform built, will do differently or see differently. A refactor, a package
  move, a renamed symbol, a docs pass, a test change: all belong to the changelog and none
  belong here. If a reader cannot act on a line, it is not a release note.

Nothing is committed here — the worktree must still be clean when step 3 runs, and the
changelog is complete precisely because no release-prep commit exists to be missing from it.

## 3. Cut the tag

```sh
go run . release --patch      # --minor / --major as the change warrants
```

Consuming repos run `./platform release --patch` instead — never a `platform` off `$PATH`.
It prints the changelog for a confirm; `ALWAYS_YES=1` clears that confirm in a TTY-less
shell. `Create` writes an annotated tag **and pushes it** to the `gh` remote — there is no
local-only mode, so this step is the point of no return.

## 4. Push the branch

```sh
git push gh main
```

The tag push carried its objects, but no branch ref moved with it. Until this runs, `main`
on the remote is behind its own tag.

## 5. Re-record the golden

```sh
./test.sh                     # expect CHANGED
./test.sh --commit
git add tests.lock.yml && git commit && git push gh main
```

The scaffolded launcher embeds `PLATFORM_VERSION`, and the `init` testbeds snapshot that
launcher, so a new tag drifts `tests.lock.yml` by exactly that pin. **Read the diff before
committing it:** the version pin is the only line a release may move, and anything else in
that diff is a regression the release just shipped, not release noise. Skipping this leaves
smoke red on `main` for whoever runs it next.

## 6. Publish the release

```sh
gh release create v0.9.17 --verify-tag \
  --title v0.9.17 \
  --notes-file /tmp/v0.9.17-release.md
```

`--verify-tag` refuses to invent a tag that does not exist, which is what catches a step-3
that never ran. The release page is the canonical home of the text from here on — edit it
there, not in a file.

## What this does not do

Cutting a release publishes no image. `release` and `publish` are orthogonal — see
[`../spec/releases.md`](../spec/releases.md). A released-but-unpublished version is a fine
state, and shipping the image is a separate `publish` run.
