# Releases

Status: **implemented.** Describes the `releases/` subsystem — the naming strategies, the
two-step generate/create flow, and how release relates to publish.

`releases/` owns exactly one concern: **cutting a named marker into git history.** It
builds nothing and pushes no image — that is `publish`'s job (see
[Orthogonality](#orthogonality-release-is-not-publish)). A release is a git tag plus the
changelog of commits since the previous one.

## The two-step flow

`platform release` runs `Generate` then, on confirmation, `Create` (`cmd/release.go`).

`Generate(cfg, git, opts)` computes the release **without mutating anything**:

1. `checkGitStatus` — reject a dirty worktree unless `Options.Force`.
2. `Recover` — fetch remote tags, list the local `v*` tags into a `Collection`.
3. `FindStrategy(cfg.Strategy)` — resolve the naming strategy.
4. `prevName = collection.LatestName(strat)` — the newest existing name the strategy
   recognizes (empty on a first release).
5. List commits in `prevName..HEAD` (all commits when there is no previous name).
6. `strat.NextName(prevName, opts.Bump)` — the name to cut.
7. Return a `Release{Name, Message, Commits}`; `Changelog` prints it for the confirm prompt.

`Create(cfg, git, rel)` performs the git mutation:

1. `UpdateAllTags` — fetch remote tags again (another machine may have pushed since).
2. `SetVersionTag(name, message)` — create an **annotated** tag (`git tag -a -m`).
3. `PushVersionTag(name)` — push it to the tracking remote.

The two are split so the plan is reviewable before any tag is written — `Generate` is pure
read, `Create` is the only writer.

## Strategies

`cfg.strategy` selects one of four (`knownStrategies`, `releases.go`). Each implements
`Strategy` — `IsValid` / `NextName` / `IsVersioned`.

| Strategy    | Name format         | Example          | First release | Increment                              |
| ----------- | ------------------- | ---------------- | ------------- | -------------------------------------- |
| `semver`    | `vMAJOR.MINOR.PATCH`| `v1.4.2`         | `v0.1.0`      | bump the requested field               |
| `datestamp` | `vYYYYMMDD[-N]`     | `v20260710-2`    | today's date  | same-day → `-N` counter, else new date |
| `timestamp` | `vYYYYMMDDHHMM`     | `v202607101432`  | now (minute)  | always `Now()` at minute precision     |
| `rolling`   | `latest` (constant) | `latest`         | `latest`      | never — single fixed name              |

### semver (`semver.go`)

Backed by `golang.org/x/mod/semver`. `NextName` canonicalizes the previous name and bumps
one field per `Bump`: `BumpPatch` (`x.y.Z+1`), `BumpMinor` (`x.Y+1.0`), `BumpMajor`
(`vX+1.0.0`). `BumpAny` (and empty) defaults to a patch bump. The `release` flags map
straight onto these — `-p`/`-m`/`--major`.

### datestamp (`datestamp.go`, `dateref/`)

`dateref.DateRef` is a date plus an integer counter; the format is `v` + `YYYYMMDD`, with
a `-N` suffix only when the counter is > 0 (`dateref.go`). `NextName`: no previous name →
`Now(0)`; previous name is today → `NextCounter()` (so a second release the same day
becomes `-1`, then `-2`, …); previous name is an earlier date → `Now(0)` (fresh date, no
counter). `dateref.Parse` reads the `^v([0-9]{8})(-[0-9]+)?$` grammar back into a
`DateRef`.

### timestamp (`timestamp.go`, `timeref/`)

Minute-precision instant, `v` + `YYYYMMDDHHMM` (`timeref.go`, format `v200601021504`,
grammar `^v([0-9]{12})$`). `NextName` ignores the previous name entirely — every release
is just `timeref.Now()`. `timeref` is name-only: no parsed struct, just `Now` / `IsValid`.

### rolling (`rolling.go`)

The **non-versioned** strategy: it never increments a version, and its one emitted name is
the conventional Docker moving tag `latest`. It exists for delivery with no versions to cut
— its moving marker is the registry image tag, not a git tag, so publishing *is* the
deploy. (The `Infra` framework seeds it, since a rendered-manifest image has no versions to
cut, and Flux follows the moving tag.) `IsVersioned` reports `false`; a versioned strategy
derives its publish target from the newest git tag, whereas `rolling` resolves its name
from the strategy directly and cuts no git tag.

## Collection — recovering history from tags

`Collection` (`collection.go`) is git's tag list, strategy-agnostic. `Recover` fetches
remote tags, lists `v*`, and reverse-sorts the names lexicographically.
`LatestName(strat)` returns the first name the strategy's `IsValid` accepts (or `names[0]`
when `strat` is nil) — this is how a repo carrying mixed tag formats still resolves the
right predecessor for its configured strategy. `Get` / `GetLatest` read a tag's annotated
message back into a `Release`; `PendingChanges` lists commits since the newest tag of any
format.

## Changelog

`generateMessage` builds the annotated-tag body: the name as a title, then one bullet per
commit — `* [<hash>][<repository>/commit/<hash>] <subject>`. Commits come from
`git log --pretty="%h %s"` over the range, parsed by `parseLogOutput` (`releases.go`).

## Tags are version tags

Every tag `releases` cuts is a **version tag**: annotated (`git tag -a`, carrying the
changelog) and pushed once, non-forcefully, to the tracking remote (`git/context.go`). A version tag is an immutable marker in history — it is never moved or
force-pushed.

The non-versioned `rolling` strategy cuts **no git tag at all**. Its moving marker is the
registry image tag, overwritten on each `publish` — an environment-style
pointer that lives in the registry, not in git. So git holds only immutable version tags;
the moving reference is a registry concern, not a force-pushed tag.

## Orthogonality: release is not publish

`release` (cut a tag) and `publish` (build + push an image) are **orthogonal — neither
implies the other**, and there is **no `deploy` verb**. Cutting a tag produces a marker in
git and nothing in the registry; producing an image is `publish`'s sole job. Under a CI
model the two co-occur (a tag triggers a build), but that is the trigger *mechanism*, not
the domain — the tag publishes nothing. A release that is never published is a fine state,
allowed by convention with no guard. Full rationale:
[delivery-verbs-are-orthogonal](../decisions/2026-07-05-delivery-verbs-are-orthogonal.md).
