# PRODIGY9 PLATFORM

`platform` is PRODIGY9's self-contained build and delivery tool: one Go binary that
detects what kind of project it's in, builds container images for it via Dagger, cuts
releases from git tags, and — for infrastructure repos — renders and ships the whole
cluster's GitOps tree.

Primary goals, unchanged since day one:

1. Eliminate build configuration from projects, or minimize it as far as feasible.
2. Let a new project bootstrap into a working CI/CD flow as fast as possible.
3. No tech-stack lock-in — adopting the next stack should be as quick as the last one.

## Quickstart

The only requirement is a recent Go toolchain (`go.mod` names the exact version;
Go projects being built must target Go 1.21+ for `GOTOOLCHAIN` pinning).

```sh
go run platform.prodigy9.co@latest init   # scaffold platform.toml + the ./platform launcher
./platform build                          # build the container image locally
./platform release -p                     # cut + push the next release tag
./platform publish                        # build + push the image for the release
```

`init` discovers the project's framework (Go, pnpm, Dockerfile, or a workspace of them —
an `infra` repo gets the whole GitOps baseline) and writes a `platform.toml` plus a
version-pinned `./platform` launcher, so collaborators never install anything.

## Commands

| Command     | What it does                                                        |
| ----------- | ------------------------------------------------------------------- |
| `init`      | Scaffold the repo from its discovered framework (alias `scaffold`). |
| `build`     | Build container image(s) via Dagger.                                |
| `preview`   | Build and serve the container locally.                              |
| `exec`      | Run a command in (or shell into) the built container.               |
| `export`    | Build and export the image as a `.docker` tarball.                  |
| `ls`        | Show the source tree going into the container.                      |
| `release`   | Cut the next release tag (`-p` patch, `-m` minor, `--major`).       |
| `publish`   | Build and push the image under the release's tag.                   |
| `render`    | Render an infra repo's `apps/` tree to `k8s/` manifests.            |
| `configure` | Print the effective parsed config.                                  |
| `clean`     | Prune the local Dagger build cache (first-line cache diagnostics).  |

`release` and `publish` are deliberately orthogonal — neither implies the other, and
there is no `deploy` verb: deployment is committing an image ref into the infra repo,
whose own `publish` ships the rendered manifests as an OCI artifact that Flux pulls and
applies. The infra repo's git history is the deployment record.

## Documentation

[`docs/`](docs/) holds the durable record, routed by [`docs/README.md`](docs/README.md):

- [`docs/spec/architecture.md`](docs/spec/architecture.md) — the build-pipeline design;
  entrypoint to the specs.
- [`docs/guides/cluster-bringup.md`](docs/guides/cluster-bringup.md) — fresh cluster to a
  live GitOps baseline, end to end.
- [`docs/guides/authoring-platform-dsl.md`](docs/guides/authoring-platform-dsl.md) —
  writing `.platform` manifest-patch files, including
  [editor setup](docs/guides/authoring-platform-dsl.md#editor-setup) (Vim/Neovim files
  ship in [`editor/nvim/`](editor/nvim/)).
- [`docs/decisions/`](docs/decisions/) — dated design rulings.

## Testing

- `go test ./...` — hermetic unit tests; also run inside every image build (green tests
  are a hard, non-configurable gate of every build).
- `./test.sh` — blackbox smoke against the testbeds (needs Docker); a drift detector
  recording golden output in `tests.lock.yml`.
