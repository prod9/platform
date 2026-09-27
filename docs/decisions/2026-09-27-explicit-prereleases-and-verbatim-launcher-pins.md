# Explicit prereleases and verbatim launcher pins

- **Date:** 2026-09-27
- **Status:** accepted

## Decision

Release names may be explicitly supplied as full SemVer, including prereleases.
Default/patch finalizes a prerelease at its existing core. Generated launchers preserve
the running binary's valid SemVer metadata verbatim, without predecessor recovery or
publication checks. Current contracts: [releases](../spec/releases.md) and
[scaffolding](../spec/scaffolding.md).

## Rationale

Manual trial versions allow alpha/beta testing without adding channel counters or release
tracks. Finalization preserves the version under test for its stable release; chakrit
chose "finalize is fine."

Launcher generation targets publicly released versions. Chakrit ruled that it should
"use the semver it can find, and trust that" and requested documentation that an
in-development build may generate a pin to a version that does not exist publicly.
Development testing invokes a binary installed at the desired commit directly.

The [v0.9-line ruling](2026-07-16-v0.9-line-is-platformv2.md) still keeps platformv2 on
`v0.9.x`. Its historical lack-of-prerelease rationale is superseded by explicit names;
trial tags use the next intended patch core. This decision authorizes no release or push.
