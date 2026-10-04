# Releases

Status: **implemented.** Includes explicit names and prerelease finalization.
Describes the `releases/` subsystem — naming
strategies, the two-step generate/create flow, and how the local CLI release command
relates to the local publish command.

`releases/` owns exactly one concern: **cutting a named marker into git history.** It
builds nothing and pushes no image — that is local `./platform publish`'s job (see
[Orthogonality](#orthogonality-release-is-not-publish)). A release is a git tag plus the
changelog of commits since the previous one.

This subsystem belongs to the local CLI driver. Its `v*` collection and configured naming
strategies do not define which GitHub pushes the CI/CD server builds or publishes; server
cadence is specified in [`execution-modes.md`](execution-modes.md).

## The two-step flow

`platform release [name]` runs `Generate` then, on confirmation, `Create`
(`cmd/release_cmd.go`).

The command accepts at most one positional name. A name and any explicitly supplied
`--patch`, `--minor`, or `--major` flag are mutually exclusive, including a flag supplied
with `=false`. Multiple supplied bump flags are also mutually exclusive, even when false.
Reject argument conflicts,
invalid names, and unsupported naming strategies before fetching tags or prompting.

`Generate(cfg, git, opts)` computes the release without creating or pushing a tag:

1. Resolve the naming strategy and validate a request carrying either a bump or an
   explicit name, never both.
2. `checkGitStatus` — reject a dirty worktree unless `Options.Force`.
3. `Recover` — fetch remote tags, list the local `v*` tags into a `Collection`.
4. `prevName = collection.LatestName(strat)` — the newest existing name the strategy
   recognizes (empty on a first release).
5. List commits in `prevName..HEAD` (all commits when there is no previous name).
6. Resolve the requested name: preserve an explicit name, or bump from `prevName`.
7. Return a `Release{Name, Message, Commits}`; `Changelog` prints it for the confirm prompt.

`Create(cfg, git, rel)` performs the git mutation:

1. `UpdateAllTags` — fetch remote tags again (another machine may have pushed since).
2. `SetVersionTag(name, message)` — create an **annotated** tag (`git tag -a -m`).
3. `PushVersionTag(name)` — push it to the tracking remote.

The two are split so the plan is reviewable before any tag is written. Recovery fetches
remote refs; `Create` owns tag creation and push. Existing tag collisions remain Git
errors; there is no overwrite mode.

## Strategies

`cfg.strategy` selects one of four (`knownStrategies`, `releases.go`). Each implements
`Strategy` — `IsValid` / `NextName` / `IsVersioned`.

| Strategy    | Name format                | Example           | First release | Increment                              |
| ----------- | -------------------------- | ----------------- | ------------- | -------------------------------------- |
| `semver`    | `vMAJOR.MINOR.PATCH[-PRE]` | `v1.4.2-alpha.1`  | `v0.1.0`      | stable bump or prerelease finalization |
| `datestamp` | `vYYYYMMDD[-N]`            | `v20260710-2`     | today's date  | same-day counter, else new date        |
| `timestamp` | `vYYYYMMDDHHMM`            | `v202607101432`   | now (minute)  | current minute                         |
| `rolling`   | `latest` (constant)        | `latest`          | `latest`      | none                                   |

### semver (`semver.go`)

Backed by `golang.org/x/mod/semver`. A supplied name must be a full
`vMAJOR.MINOR.PATCH`, optionally with valid prerelease identifiers, and is preserved
verbatim. Shorthand, malformed names, and build metadata are rejected. Explicit names
are supported only under `semver`; other strategies retain automatic naming.

```sh
platform release v0.9.54-alpha.1
platform release v0.9.54-beta.2
platform release v0.9.54
```

Automatic bumps operate on the numeric core and always produce stable names.
`BumpAny` (and empty) defaults to patch; default/`--patch` finalizes a prerelease at its
existing core rather than consuming the next patch. Minor and major increment the
requested field and reset lower fields. First automatic release remains `v0.1.0`.

| Previous name    | Default / `--patch` | `--minor` | `--major` |
| ---------------- | ------------------- | --------- | --------- |
| `v1.2.3`         | `v1.2.4`            | `v1.3.0`  | `v2.0.0`  |
| `v1.2.3-alpha.1` | `v1.2.3`            | `v1.3.0`  | `v2.0.0`  |
| `v1.3.0-beta.2`  | `v1.3.0`            | `v1.4.0`  | `v2.0.0`  |
| `v2.0.0-rc.1`    | `v2.0.0`            | `v2.1.0`  | `v3.0.0`  |

There are no channel counters, automatic alpha/beta transitions, separate stable and
prerelease tracks, or publication checks. For this repository, platformv2 remains on
`v0.9.x`; manually named trials use the next intended unreleased patch core. Releases of
this repository remain patch releases in v0.9 until the complete platform CI/CD
functionality works; this repository convention does not restrict consumer repositories.

The operator's governing instruction is: "cut new v0.9.XX patch releases if you need
tagged releases, stay in v0.9 until the whole platform ci/cd functionality are fully
working".

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
remote tags, lists `v*`, and sorts newest-first by name class: chronological references
by date and counter, then SemVer by semantic precedence, then other names by byte order.
`LatestName(strat)` returns the first name the strategy's `IsValid` accepts (or `names[0]`
when `strat` is nil) — this is how a repo carrying mixed tag formats still resolves the
right predecessor for its configured strategy. `Get` / `GetLatest` read a tag's annotated
message back into a `Release`; `PendingChanges` lists commits since the newest tag of any
format.

Within SemVer, `v0.9.54` sorts above `v0.9.54-alpha.1`, while `v0.9.55-alpha.1`
sorts above both. Numeric identifiers compare numerically (`alpha.10` above `alpha.2`).
Local publishing keeps this highest-version selection and uses the name verbatim as its
image tag; it does not select a stable channel separately.

## Changelog

`generateMessage` builds the annotated-tag body: the name as a title, then one bullet per
commit — `* [<hash>][<repository-url>/commit/<hash>] <subject>`, where the link is
`conf.RepositoryURL` (the `https://` form; `repository` itself is stored scheme-less — see
[architecture.md](architecture.md) §Package layout). Commits come from
`git log --pretty="%h %s"` over the range, parsed by `parseLogOutput` (`releases.go`).

**Neither this nor `Release.Changelog()` is release-note generation.** The tag body and the
terminal listing are supplementary data about a release — every commit, verbatim. Release
notes are prose a human writes: what changed for someone using the thing, and why it
matters. No command produces them and none will.

## Tags are version tags

Every tag `releases` cuts is a **version tag**: annotated (`git tag -a`, carrying the
changelog) and pushed once, non-forcefully, to the tracking remote (`git/context.go`). A version tag is an immutable marker in history — it is never moved or
force-pushed.

The non-versioned `rolling` strategy cuts **no git tag at all**. Its moving marker is the
registry image tag, overwritten on each `publish` — an environment-style
pointer that lives in the registry, not in git. So git holds only immutable version tags;
the moving reference is a registry concern, not a force-pushed tag.

## Orthogonality: release is not publish

The local `./platform release` command (cut a tag) and local `./platform publish` command
(build + push an image) are **orthogonal — neither implies the other**, and there is **no
`deploy` verb**. Cutting a tag produces a marker in git and nothing in the registry; only an
explicit local publish invocation pushes an image. This local command contract says
nothing about the server driver, which applies its own build triggers and publish cadence.
A release that is never locally published is a fine state, allowed by convention with no
guard. Full rationale: [execution-mode decision].

[execution-mode decision]: ../decisions/2026-08-24-execution-mode-does-not-define-delivery-policy.md
