# Execution modes and delivery policy

Status: **design-of-record.** This file owns the boundary between platform's shared
build/publish capabilities, the two drivers that invoke them, and the opinionated
delivery workflow built on top. A command, a server event, and a repository convention
are different things even when they eventually call the same engine operation.

## Four layers that must never be collapsed

| Layer                     | Question it answers                                         |
|---------------------------|-------------------------------------------------------------|
| Shared capability         | What build or publish operation can the engine perform?     |
| Execution mode            | Who invokes the operation, from where, and when?            |
| Intended delivery policy  | Which successful builds should produce registry images?    |
| This repository's runbook | How does `prod9/platform` itself cut and announce releases? |

The first three describe the product. The fourth is an operator procedure for this one
repository. A rule in
[`../guides/cutting-a-release.md`](../guides/cutting-a-release.md) is not a restriction
on repositories that consume platform.

## Shared capabilities are not an execution mode

The shared packages parse `platform.toml`, interpret modules into build units, build those
units, and optionally publish their images. They know neither why a run started nor which
repository event should publish. `engine.Build` and `engine.BuildAndPublish` are reusable
operations; cadence and policy belong to their caller.

`release` is separate again. The releases subsystem can cut a named git marker. It does
not build or publish, and the server does not need to use platform's release-naming
strategies to recognize an ordinary git tag.

## Local CLI mode

Local CLI mode is an operator synchronously invoking the version-pinned launcher in a
checked-out workdir:

- `./platform build` builds the selected modules and does not push an image.
- `./platform release` explicitly cuts the next tag selected by `platform.toml`'s release
  strategy.
- `./platform publish` explicitly builds and pushes the selected modules; the configured
  release strategy resolves the image tag.

The CLI performs no watching and owns no automatic cadence. Running
`./platform publish` is a human or script choosing to publish now. It is not the
operation the server performs merely relocated into a pod, and the CLI's `v*` release
conventions do not filter server builds.

## CI/CD server mode

The CI/CD server is one execution mode with two cooperating processes:

- `platform srv` authenticates incoming signals and records immutable build intent.
- `platform worker` asynchronously claims that intent, prepares the exact repository
  snapshot, and invokes the shared build or build-and-publish capability.

Together they are the server driver, peer to the local CLI driver. The HTTP request is
not the build lifetime, the worker is not a second mode, and the server never shells out
to `./platform publish`.

Every non-deleted GitHub push creates a whole-repository build at the pushed commit.
Branch pushes, default-branch pushes, and tag pushes all build; no `v` prefix is special.
A manual webui build may name any ref and select modules, but it records the same domain
intent before a worker acts.

Publishing is a policy applied to a successful server build:

| Repository kind | Build cadence       | Publish cadence                      | Image tag               |
|-----------------|---------------------|--------------------------------------|-------------------------|
| App             | every pushed commit | tag-triggered successful builds only | exact git tag, any name |
| Infra           | every pushed commit | every successful build               | `latest`                |

How the server identifies the repository kind is deliberately unresolved. Until that
design is settled, no spec may infer it from release strategy, tag spelling, module
names, or framework implementation.

## Intended delivery workflow

The intended workflow is an opinionated convention over the two execution modes, not
another engine API:

- App repositories continuously validate every pushed commit. Pushing any git tag marks a
  build whose successful image is worth publishing under that exact tag.
- Infra repositories treat committed desired state as the record. Every successful build
  publishes the rendered tree to the moving `latest` tag, which Flux follows.
- Deploying an app remains an infra-repository commit that changes the literal app image
  ref. Platform has no `deploy` verb and never rewrites that desired state as a side
  effect of an app publish.

The same engine makes local and server execution related; invocation context and cadence
keep them distinct. The same publish mechanism serves app and infra repositories;
delivery policy keeps their publishing cadence distinct.
