# The CUE module path is a scaffold input read from cue.mod — not the repository, not platform.toml

Date: 2026-07-12
Status: **accepted**

## The ruling

The CUE module path an infra repo uses — `module:` in `cue.mod/module.cue`, and the prefix of
every `import "<prefix>/defaults"` — is a value the **operator supplies at init** as the
`CUE_MOD_PREFIX` scaffold input (greenfield), or is **read from an existing `cue.mod`**. It is
**not** derived from `repository`, and it is **never persisted in `platform.toml`**. `cue.mod`
is its sole home.

## Why not the repository

`platform init` once defaulted the module path to `repository`. That breaks on any repo CUE
won't accept as a module path: CUE requires the first path element to be a domain (contain a
dot), so `prod9/infra-new` failed with *"missing dot in first path element"*. The GitHub
org/repo is not a domain. `repository` (where the code is hosted) and the CUE module namespace
are separate concerns — both real infra repos prove the divergence:

| `repository`                   | CUE module (`module:`) |
| ------------------------------ | ---------------------- |
| `github.com/prod9/infra`       | `prodigy9.co`          |
| `github.com/prod9/infra-basic` | `infra-basic.test`     |

## Why not a platform.toml key

The value has one home every consumer already reads: `cue.mod` — operator truth, never
rewritten after init. `render` loads the apps by the module path read from `cue.mod`; init seeds
the app-import lines from it. A parallel `platform.toml` key would be a second source of truth
that can only drift from `cue.mod` or lie — it strengthens nothing and invites the footgun of an
edit that silently does nothing. So there is no `platform.toml` module-path key.

## Shape

- **`Framework.RequiredScaffoldInputs(wd) []string`** declares the inputs a framework needs at
  init. Infra returns `["CUE_MOD_PREFIX"]` only when greenfield (`!cuemod.Present`); an existing
  `cue.mod` is read, never re-asked. The driver prompts each by name and stays framework-agnostic
  — no app-vs-infra branch.
- **`Infra.Scaffold`** owns resolving its own files: it resolves the module path (the
  `CUE_MOD_PREFIX` input greenfield, the existing `cue.mod` otherwise — it wins) and validates it
  as a legal CUE module path (a domain in the first segment), failing clearly rather than
  deferring to CUE's cryptic error. The driver writes finished bytes and never sees a template hole.
- The operator **never hand-edits `cue.mod`**: the value is asked once, at init, and flows to
  `cue.mod` + the app imports together.

## Naming

`CUE_MOD_PREFIX` names exactly what it drives — the `cue.mod` module and the prefix of every app
import; the input name is its prompt label. It is a scaffold-time input, deliberately distinct
from `[vars]` (the persisted render-interpolation table), from `repository` (GitHub host), and
from `[modules]` (platform's own build units).
