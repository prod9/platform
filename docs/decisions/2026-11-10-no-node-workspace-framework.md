# No Node workspace framework

- **Date:** 2026-11-10
- **Status:** accepted

## Decision

`pnpm/workspace` is removed: runner, discovery, catalog entry, testbed and smoke golden.
Platform builds no Node workspace. A pnpm workspace root falls through to `pnpm/basic` on
its lockfile, and a `platform.toml` naming `pnpm/workspace` fails as an unknown
framework. No replacement is prescribed. Current contract:
[frameworks](../spec/frameworks.md).

chakrit ruled it: "completely remove pnpm/workspaces builder and leave a note in specs or
otherwise noting the myraid compat problem we'd be having plus that node eco system note
thing". The framework had been withdrawn from the intended catalog since 2026-10-04; no
naxon-ai repository used it (naxon's Go modules use `go/workspace`, which stays).

## Rationale

The obvious move is to keep a workspace framework and fix it. It was not fixed because
every part of a Node workspace build is tool-specific, and the one runner platform had
did the single thing a runner must not do.

**The runner copied `node_modules` between images.** It lifted a member's `node_modules`
out of the build container into the runner. `node_modules` belongs to the environment
that ran the install: native addons link against that image's libraries, and pnpm's
layout is links into a store at paths of that filesystem. Fixing this
means installing in the runner, which for a workspace needs the root lockfile, the
workspace file, every member's manifest, and the source and build output of every
member it depends on through `workspace:` — the whole workspace rebuilt in the runner,
with the app moving to `RunDir/<member>`.

**The ecosystem has no common workspace contract to build against** (checked
2026-11-10):

- *Declaring a workspace.* npm, Yarn and Bun read a `workspaces` array in the root
  `package.json`. pnpm reads only `pnpm-workspace.yaml`. Its maintainer declines the
  field on semantics — in pnpm a workspace is the set of projects, in Yarn each package
  is a workspace — and on lookup cost. pnpm 12.4.1 warns when the field is present
  without the file, and pnpm 12.7.0 generates `pnpm-workspace.yaml` from it once
  (pnpm/pnpm#2255, closed 2026-09-26 as "we have added autoconversion"). pnpm keeps
  reading only its own file, which since v10 also holds every non-auth setting.
- *Shipping one member.* No cross-tool command exists. pnpm has
  `pnpm deploy --filter <pkg> --prod <dir>`; npm has nothing equivalent. pnpm's own
  Docker guidance builds that directory in one stage and copies it into another — the
  copying this repository rejects.
- *Choosing the package manager.* Corepack is no longer bundled with Node from v25, and
  the Node 26 Docker images ship neither Corepack nor Yarn. pnpm 12 itself switches to
  the version pinned in `packageManager` or `devEngines.packageManager`.

A workspace framework would therefore be a pnpm-only design built on pnpm-only commands,
carrying the cross-tool compatibility gaps above for every repo that is not pnpm.

**What a replacement must settle before it is designed:** how the runner installs a
member's dependencies in its own image, how `workspace:` dependencies on sibling members
reach the runner, which package managers it supports and how it detects them, and what
`RunDir` holds for a member. The 2026-10-04 condition still stands: build it only when
a concrete need establishes it.
