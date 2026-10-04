<!-- not spec/decision because: implementation handoff; current contracts live in spec/ -->

# Explicit prerelease versions — analysis and implementation plan

Status: completed on 2026-09-27 in `496eaab` (specification), `8621634`
(implementation), and `55d9292` (table alignment). Audience: engineers inspecting the
historical feature plan; this is not an implementation queue. Current contracts live in
[releases.md](../spec/releases.md), [scaffolding.md](../spec/scaffolding.md), and
[testing.md](../spec/testing.md); verification is in `tmp/prerelease.local/report.md`.
The behavior described as current below is the pre-implementation observation.

## Goal and authority

Support manually named releases such as `v0.9.54-alpha.1` and `v0.9.54-beta.2`,
including using prerelease builds of platform itself. Ordinary bumps must produce
stable versions without parsing failures or stray suffix fragments.

User request: "analyze and plan for this support first" and "no need to add support
for bumping into these trail versions". This document is planning only; no application
code, tags, remote state, or current specifications were changed during that analysis.
Subsequent authority: "we'll implement the plan next session", then "start" after the
specification, implementation, and verification sequence was presented. The user also
authorized working without gopls MCP: "no gopls mcp tool, work without it for now."

## Verified current behavior

- `cmd/release_cmd.go:15` advertises a name, but `runReleaseCmd` at line 40 never reads
  its positional arguments; lines 47–66 request an automatic bump every time.
- `releases/releases.go:68` owns generation: recover history, choose the previous
  release, collect commits, select the next name, and format its changelog.
  `Create` at line 106 owns tag creation and push through the existing Git methods.
- `releases/semver.go:17` delegates validity to x/mod SemVer. At lines 29–60, bumping
  splits the entire canonical version on dots. For `v1.2.3-alpha.1`, patch parsing
  fails on `3-alpha`; minor and major leave an extra `.1` in their output.
- `releases/collection.go:81` already compares SemVer names using `semver.Compare`.
  `LatestName` at line 113 selects the highest accepted version, including prereleases.
  `docs/spec/releases.md:86` incorrectly describes this as lexicographic ordering.
- `cmd/publish_cmd.go:45` selects that latest release and passes its name at line 66
  through `engine/session.go:71` to `engine/publication.go:35`, which appends it
  verbatim as the image tag; no suffix transformation is needed for alpha/beta names.
- `framework/version.go:26` accepts only stable exact tags and stable-derived Go
  pseudo-versions. An unrecognized version causes `scaffolding/plan.go:98` to fail
  launcher resolution. The approved launcher behavior below replaces this restricted
  recognition and predecessor recovery with verbatim SemVer handling.
- `framework/skel/platform.tmpl:5` carries the version into the executable filename
  and exact `go install` argument; `cmd/versions_cmd.go:25` displays the raw build
  version. Neither needs a new channel mechanism.

## Proposed CLI contract

Under `strategy = "semver"`:

```sh
platform release v0.9.54-alpha.1
platform release v0.9.54-alpha.2
platform release v0.9.54-beta.1
platform release v0.9.54
```

These examples describe future behavior, not commands to run during planning.

- Accept zero or one positional name. A supplied name and any bump flag are mutually
  exclusive; reject conflicts and extra arguments before fetching tags or prompting.
- Require the full `vMAJOR.MINOR.PATCH` form, optionally with a SemVer prerelease
  suffix. Preserve the supplied name exactly; reject malformed names, shorthand,
  and build metadata rather than silently normalizing them.
- Accept valid prerelease identifiers generally, including `alpha.1`, `beta.2`, and
  `rc.1`; do not introduce a channel enum, channel configuration, or counter generator.
- Limit explicit names to the SemVer strategy in this slice. Other strategies retain
  their existing automatic behavior and reject an explicit name clearly.
- Retain the existing changelog, confirmation, dirty-tree check, annotated tag, and
  push path. Existing tag collisions remain Git errors; no overwrite mode is added.

The full-name requirement keeps the contract unambiguous. Build metadata support is
outside this prerelease task; it must not be discarded from a user-supplied name.

## Proposed bump behavior

Approved: default/`--patch` finalizes a prerelease at its existing numeric version,
so testing a version does not consume that version before its stable release.
Authority: chakrit answered "finalize is fine" when asked whether
`v1.2.3-alpha.1` should become `v1.2.3` or `v1.2.4`.
The implementation plan retains literal numeric minor/major increments.

| Previous highest version   | Default / `--patch`  | `--minor`   | `--major`   |
| -------------------------- | -------------------- | ----------- | ----------- |
| `v1.2.3`                   | `v1.2.4`             | `v1.3.0`    | `v2.0.0`    |
| `v1.2.3-alpha.1`           | `v1.2.3`             | `v1.3.0`    | `v2.0.0`    |
| `v1.3.0-beta.2`            | `v1.3.0`             | `v1.4.0`    | `v2.0.0`    |
| `v2.0.0-rc.1`              | `v2.0.0`             | `v2.1.0`    | `v3.0.0`    |

Minor and major remain literal numeric increments with lower fields reset. Every
automatic result is stable. No automatic `alpha.1 → alpha.2`, alpha-to-beta, or
stable-to-prerelease transition is introduced. First automatic release remains `v0.1.0`.

## Selection and publishing convention

Keep highest-SemVer selection; do not add separate stable/prerelease tracks or a
publish selector. A stable `v0.9.54` sorts above `v0.9.54-alpha.1`, while
`v0.9.55-alpha.1` sorts above both. Therefore testing should use the next intended
unreleased base, rather than adding an alpha suffix to an already stable release.

This is ordinary SemVer precedence, not a rule about tag creation time; see the
[SemVer specification](https://semver.org/#spec-item-11).

## Approved launcher behavior

The generated launcher pins the valid SemVer version found in the running binary's
build metadata verbatim. Use SemVer validation only: do not recover a predecessor,
strip suffixes, or add special handling for Go pseudo-versions or development builds.
Missing or invalid SemVer remains an error; a valid value is trusted without checking
whether it has been published.

Document this scope in the scaffolding specification: launcher generation targets
publicly released builds, versions, and tags. On an in-development build, the generated
pin may point to a version that does not exist publicly. This limitation is documented,
not addressed with a fallback or a development mode in the launcher.

Development testing uses a separate path, such as installing platform at the desired
commit with `go install platform.prodigy9.co@<commit>` and invoking the installed binary
directly rather than using a pinned launcher.

Authority: chakrit confirmed this understanding with "correct. amend the plan please".
Launcher behavior and the default/patch choice above are settled independently.

## Implementation sequence after approval

1. **Specification slice.** Update `docs/spec/releases.md` with explicit-name syntax,
   validation, the agreed bump table, and actual selection behavior; update
   `docs/spec/scaffolding.md` for verbatim SemVer pins and the released-build scope
   documented above, replacing its predecessor-release requirement.
   Record the change from the historical patch-only rationale in
   `docs/decisions/2026-07-16-v0.9-line-is-platformv2.md` through a new ruling if needed,
   preserving the historical ADR. Update release-runbook examples for manual trial
   versions and GitHub prerelease designation without choosing or cutting a release.
2. **Release implementation slice.** Adapt the existing release command and generation
   path. Resolve a request as either an explicit validated name or a bump, rather than
   passing independent contradictory name/bump fields through the domain. Reuse the
   existing generation and mutation owners. Separate the numeric core from the
   prerelease suffix before arithmetic; never split the complete suffixed version into
   numeric fields. Keep this behavior in `releases`, not in the command adapter.
3. **Launcher compatibility in the same feature work.** Replace the restricted regex
   and manual pseudo-version decrement in `framework/version.go` with SemVer validation
   and return the valid build-metadata version unchanged. Remove predecessor recovery
   and dirty-suffix stripping; add no `x/mod/module` handling. Keep errors for missing
   or invalid versions. The launcher template continues to use the supplied pin.
4. **Verification and audit.** Run the focused behavior checks and complete repository
   suites below, review smoke drift, then audit the complete feature against the
   approved specs. No release or publication belongs to this implementation task.

The existing dependency is `golang.org/x/mod v0.33.0` (`go.mod:17`); no new dependency
is proposed. Its public [SemVer APIs](https://pkg.go.dev/golang.org/x/mod/semver)
provide validation and suffix extraction for releases. Launcher generation uses only
validation, because canonicalization or suffix removal would change the supplied pin.

## Verification plan

- Exercise explicit alpha/beta/stable names, malformed names, extra arguments, and
  name-plus-bump conflicts through the CLI boundary; verify rejection precedes effects.
- Test the agreed bump table, stable behavior, first release, and suffixes with several
  dotted identifiers. These protect semantic transitions, not parser implementation.
- Extend release-collection coverage with `alpha.2`, `alpha.10`, `beta.1`, stable,
  and a newer-core prerelease to verify the selection contract used by publishing.
- Update `framework/version_test.go` to verify stable and prerelease versions pass
  through unchanged and missing/invalid versions fail. Replace predecessor-recovery
  expectations with the verbatim contract, including valid suffixed metadata; do not
  introduce development-specific behavior or tests of Go pseudo-version internals.
- Add a blackbox release fixture to the existing smoke suite using an isolated local
  repository and local bare `gh` remote, exercising named releases, tag contents,
  collisions, and the subsequent stable bump without reaching a shared remote.
- Run `go test ./...` and `./test.sh`; read and deliberately record expected smoke
  changes, preserving all existing timeout budgets. Do not run suites during planning
  merely to claim an implementation that does not yet exist.

## Review state

Analysis used source inspection and a read-only `go version -m ./bin/platform` check;
no release was run. The repository was clean before this planning document. The user
subsequently authorized implementation as recorded under Goal and authority.

Verification refinement: compare the generated launcher's version pin exactly with the
generating binary's version, then snapshot a normalized projection without changing the
generated file. Verbatim metadata otherwise makes the golden drift on each commit.
The current contracts are `docs/spec/releases.md`, `docs/spec/scaffolding.md`, and
`docs/spec/testing.md`; they precede implementation.
