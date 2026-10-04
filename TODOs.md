# Pending work

Current implementation state belongs to the ACE save and ledger under `.ace/`; durable
contracts belong to [docs/spec/](docs/spec/). The remaining historical proposals are
listed in [PLANS.md](PLANS.md) and require fresh scope verification.

First-release naming, fetching remote tags, and the tracking-remote contract are specified
in [releases.md](docs/spec/releases.md). Platform-managed environments and an environment
command are superseded by the [execution-mode contract](docs/spec/execution-modes.md)
and [migration guide](docs/guides/migration.md).

The old cross-machine Dockerfile synchronization and logging notes have no current
acceptance criteria or verified pending state; retain them as unconfirmed historical
leads only, not actionable tasks.
