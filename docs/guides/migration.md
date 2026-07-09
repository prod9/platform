# Migration — legacy platform → pull-based v2

Living checklist of breaking changes as the pull-based rework lands, and how a consuming repo
moves across each. Newest first.

## Environments are no longer platform-managed

The `environments` list (`platform.toml`) and the `deploy` verb are **removed**. Platform no
longer owns any notion of an environment.

Multi-environment is now expressed **manually in the infra repo's CUE** — value templating +
k8s namespacing: a shared template (e.g. `apps/template/`) instantiated once per environment
(`apps/dev`, `apps/prod`, …), each landing in its own namespace. No platform enum, no
`platform deploy`.

Migrate:
- Drop `environments = [...]` from `platform.toml`.
- Model each environment as a CUE instance of a shared template, separated by namespace.
- Deploy = commit the infra repo, then `publish` (with a platform server + Flux) **or**
  `render` + `kubectl apply` (no server). The gate is the infra repo's GitHub push
  permissions — whoever can push can deploy.
