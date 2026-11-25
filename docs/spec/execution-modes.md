# Execution modes and delivery policy

Status: **design-of-record; v1.0 rescope intended.** This file owns the boundary between
platform's shared build/publish capabilities, the CLI that invokes them, and the
opinionated delivery workflow built on top. A command and a repository convention are
different things even when they eventually call the same engine operation.

The [execution-mode decision] records the ruling behind this separation; the
[server-removed decision] leaves the CLI as platform's one execution mode.

[execution-mode decision]: ../decisions/2026-08-24-execution-mode-does-not-define-delivery-policy.md
[server-removed decision]: ../decisions/2026-11-25-server-removed-platform-is-a-cli.md

## Four layers that must never be collapsed

| Layer                     | Question it answers                                         |
|---------------------------|-------------------------------------------------------------|
| Shared capability         | What build or publish operation can the engine perform?     |
| Execution mode            | Who invokes the operation, from where, and when?            |
| Intended delivery policy  | Which successful builds should produce registry images?     |
| This repository's runbook | How does `prod9/platform` itself cut and announce releases? |

The first three describe the product. The fourth is an operator procedure for this one
repository. A rule in
[`../guides/cutting-a-release.md`](../guides/cutting-a-release.md) is not a restriction
on repositories that consume platform.

## Shared capabilities are not an execution mode

The engine facade owns the shared workdir-to-image capability: config parsing, module
interpretation, runner placement, execution, and optional publication. It knows neither
why a run started nor which builds should publish. Cadence and policy belong to its
caller. [`engine.md`](engine.md) specifies that capability.

`release` is separate again. The releases subsystem can cut a named git marker. It does
not build or publish. [`releases.md`](releases.md) specifies it.

## Local CLI mode

Local CLI mode is an operator synchronously invoking the version-pinned launcher in a
checked-out workdir:

- `./platform build` builds the selected modules and does not push an image.
- `./platform release` explicitly cuts the next tag selected by `platform.toml`'s release
  strategy.
- `./platform publish` explicitly builds and pushes the selected modules; the configured
  release strategy resolves the image tag.
- `./platform render` writes an infra repo's rendered manifest tree to `k8s/` and pushes
  nothing.

The CLI performs no watching and owns no automatic cadence: platform runs no server, no
webhook receiver, and no build queue. Running `./platform publish` is a human or script
choosing to publish now. A script that runs the launcher is the same mode; how often it
runs, and on which commits, is that script's choice and not a platform setting.

## Intended delivery workflow

The intended workflow is an opinionated convention over the CLI, not another engine API
and not a `platform.toml` setting.

- App repositories publish released versions. The operator cuts a release with
  `./platform release` and publishes it with `./platform publish`; the release strategy
  resolves the image tag.
- Infra repositories treat committed desired state as the record. They use the `rolling`
  strategy, so `./platform publish` pushes the rendered tree under the moving `latest`
  tag, which Flux follows. A cluster without that path takes `./platform render` +
  `kubectl apply` instead.
- Deploying an app remains an infra-repository commit that changes the literal app image
  ref. Platform has no `deploy` verb and never rewrites that desired state as a side
  effect of an app publish.

The same publish mechanism serves app and infra repositories; their release strategies
keep their image tags distinct.
