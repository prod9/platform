# Execution modes and delivery policy

Status: **design-of-record.** This file owns the boundary between platform's shared
build/publish capabilities, the two drivers that invoke them, and the opinionated
delivery workflow built on top. A command, a server event, and a repository convention
are different things even when they eventually call the same engine operation.

The [execution-mode decision] records the ruling behind this separation.

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

[execution-mode decision]: ../decisions/2026-08-24-execution-mode-does-not-define-delivery-policy.md

## Shared capabilities are not an execution mode

The engine facade owns the shared source-to-image capability: repository materialization
when needed, config parsing, module interpretation, runner placement, execution, and
optional publication. It knows neither why a run started nor which repository event should
publish. Cadence and policy belong to its caller. [`engine.md`](engine.md) specifies that
capability.

`release` is separate again. The releases subsystem can cut a named git marker. It does
not build or publish, and the server does not need to use platform's release-naming
strategies to recognize an ordinary git tag.

[`releases.md`](releases.md) specifies the local release subsystem; it is not a server
trigger specification.

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
- `platform worker` asynchronously claims that intent, constructs an immutable
  remote-build request, and invokes the engine facade once.

Together they are the server driver, peer to the local CLI driver. The HTTP request is
not the build lifetime, the worker is not a second mode, and the server never shells out
to `./platform publish`. [`platform-server.md`](platform-server.md) specifies this mode.

Every non-deleted GitHub push for a **registered** repository creates a
whole-repository build at the pushed commit. Branch pushes, default-branch pushes, and tag
pushes all build; no `v` prefix is special. An App-installed repository that has not been
registered is not admitted to the build queue. A manual webui build may name any ref and
select modules, but it records the same domain intent before a worker acts.

Publishing is a policy applied to a successful server build. Each `platform.toml` may
declare it independently of the local release strategy:

```toml
[server]
publish = "always" # "always" | "tags" | "never"
```

| Policy   | Publish cadence                      | Image tag               |
|----------|--------------------------------------|-------------------------|
| `always` | every successful build               | `latest`                |
| `tags`   | tag-triggered successful builds only | exact git tag, any name |
| `never`  | never                                | none                    |

When `server.publish` is absent, a case-sensitive repository name matching
`(^|-)infra$` resolves to `always`; every other name resolves to `tags`. The server stores
the resolved value with the immutable manifest observation used by the build. It never
infers policy from release strategy, tag spelling, module names, or framework
implementation.

## Intended delivery workflow

The intended workflow is an opinionated convention over the two execution modes, not
another engine API. [`platform.md`](platform.md) carries the wider control-plane vision.

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
