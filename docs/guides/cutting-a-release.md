# Cutting a release

Repository runbook for `prod9/platform` itself. It takes this repository's green `main` to
a published GitHub release: a notes file, a tag, a pushed branch, a reviewed smoke result,
and a release page carrying the notes. It is not a product contract for consuming
repositories. The [execution-mode specification] owns that boundary.

In this repository, `go run .` is the platform launcher; there is no `./platform` file.
Use `go run . release` and `go run . publish` for this repository's local CLI operations.

[execution-mode specification]: ../spec/execution-modes.md

The notes are drafted **outside the repo**, in `/tmp`. Their permanent home is the GitHub
release page, so a copy in the tree is a second original that drifts, and `platform release`
refuses a dirty worktree anyway — an uncommitted draft would block the tag it exists to
describe. Commit any required reviewed golden changes separately.

## 0. Preconditions

```sh
git pull --ff-only gh main  # before any work, not only before the tag
git status --short          # must be empty
go test ./...               # must pass
./test.sh                   # must report UNCHANGED
```

🚨 **Pull first.** A release is cut from whatever local `main` holds. A clone behind
`gh/main` tags a commit missing published work, its `main` push is rejected after the tag
has already gone out, and its tests and smoke ran against the wrong tree. Pull before
writing the change, and confirm `git status -sb` shows no `behind` before step 3. If the
pull is not a fast-forward, stop and reconcile before releasing.

A `CHANGED` golden here is unreviewed drift from earlier work — settle it before releasing,
never in the same diff as the release. The launcher test compares the pin with its binary
and snapshots a normalized projection, so a version change alone should not move the
golden (step 5).

## 1. Compile the changelog

```sh
git fetch gh --tags
git tag -l 'v*' --sort=-v:refname > /tmp/tags.txt     # read the top: every line counts
git log --reverse --format='- `%h` %s' v0.10.0-alpha.1..HEAD > /tmp/v0.10.0-alpha.2-release.md
```

Read the whole top of the tag list, prereleases included. `git describe` is not a
substitute: it sees only tags reachable from `HEAD`, so it misses a newer line such as
`v0.10.0-alpha.3` and reports a stale previous version. Decide the release's exact name
here, from that list.

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
go run . release v0.10.0-alpha.1   # the exact name decided in step 1
```

The command prints the name and changelog for a confirm; `ALWAYS_YES=1` clears that
confirm in a TTY-less shell. `Create` writes an annotated tag **and pushes it** to the `gh`
remote — there is no local-only mode, so this step is the point of no return.

🚨 **Name the version; never let a bump pick it under `ALWAYS_YES`.** `--patch` finalizes
the highest prerelease at its core, so with a `v0.10.0-alpha.*` tag present it cuts the
final `v0.10.0`, not the next alpha. The confirm prompt is the only place the computed name
shows before the push, and `ALWAYS_YES=1` skips it. Pass the explicit name from step 1.

Subsequent releases increment the alpha identifier explicitly:

```sh
go run . release v0.10.0-alpha.2
```

Keep the `v0.10.0` core fixed and increment the alpha identifier. Counters are manual;
default/`--patch` finalizes the highest prerelease at its existing core. Local publish
selects the highest SemVer, including prereleases, rather than the newest tag by creation
time. Explicit names are the intended prerelease contract in
[releases.md](../spec/releases.md).

When the release contains server-side changes intended for deployment or live testing,
run `go run . publish` after cutting the release. Report only the tag-qualified image name
(`ghcr.io/prod9/platform:vX.Y.Z`) for the infra handoff; do not include the emitted digest.

## 4. Push the branch

```sh
git push gh main
```

The tag push carried its objects, but no branch ref moved with it. Until this runs, `main`
on the remote is behind its own tag.

## 5. Verify smoke after the tag

```sh
./test.sh                     # expect UNCHANGED
```

The launcher pin is checked against the generating binary before normalization for the
snapshot. If smoke reports `CHANGED`, read the complete diff and explain the drift;
do not dismiss it as version noise. Record only reviewed intended changes with
`./test.sh --commit`, then commit and push that golden through the normal authorization
boundary.

## 6. Create the GitHub release

```sh
gh release create v0.10.0-alpha.1 --verify-tag --prerelease \
  --title v0.10.0-alpha.1 \
  --notes-file /tmp/v0.10.0-alpha.1-release.md
```

`--verify-tag` refuses to invent a tag that does not exist, which is what catches a step-3
that never ran. The release page is the canonical home of the text from here on — edit it
there, not in a file.

Keep `--prerelease` for the alpha series so GitHub marks these releases as prereleases.

## Infra handoff and live verification

Agree each work slice with chakrit before implementing it. When authorized for the
current session, cut and publish `v0.10.0-alpha.N` releases as appropriate for completed,
verified slices; keep the numeric core fixed and increment only the alpha identifier.

After publication, an authorized platform agent may instruct the infra agent in
`prod9/infra` to roll out the exact published image. Use the infra subagent when one is
attached, or `tmux send-keys` to its confirmed agent pane when that communication is
authorized. The infra agent follows its own repository's deployment instructions.
Obtain chakrit's permission again in every fresh session before sending infra deployment
instructions; this runbook records the workflow, not a standing permission grant.

The platform agent may use `kubectl` in read-only mode to inspect deployment state,
events, and error logs. Rollout mutations belong to the infra agent. Once rollout is
confirmed, use the connected Chrome browser at `https://platform.prodigy9.co` to verify
the released functionality and layouts against real server responses. Ask chakrit for
help when authentication requires it. Record live results separately from local fixtures
and report missing browser access as a blocker to live verification.

## What this does not do

Cutting a release publishes no image. `release` and `publish` are orthogonal — see
[`../spec/releases.md`](../spec/releases.md). A released-but-unpublished version is a fine
state, and shipping the image is a separate `go run . publish` run in this repository.
