# CUE module path comes from `import_prefix`, not the repository

Date: 2026-07-12
Status: **accepted**

## The ruling

The CUE module path an infra repo scaffolds (`module:` in `cue.mod/module.cue`, and the prefix
of every `import "<prefix>/defaults"`) comes from a dedicated top-level `import_prefix` setting —
**not** from `repository`. The two are separate namespaces: `repository` is where the code is
hosted on GitHub; `import_prefix` is the operator's own CUE import namespace.

Both real infra repos already prove the divergence:

| `repository`                   | `import_prefix` (`module:`) |
| ------------------------------ | --------------------------- |
| `github.com/prod9/infra`       | `prodigy9.co`               |
| `github.com/prod9/infra-basic` | `infra-basic.test`          |

## Why

`platform init` was the only place the two were forced equal — `planSpecFiles` defaulted the
module path to `info.Repository`. That broke on any repository CUE won't accept as a module path:
CUE requires the first path element to be a domain (contain a dot), so `prod9/infra-new` failed
with *"missing dot in first path element"* at render. The GitHub org/repo is not a domain and
cannot be assumed to be one.

The fix is a separate setting, not a repair of the repository value: forcing `repository` to be
domain-qualified, or prepending a guessed `github.com/`, both conflate the two concerns.

## Shape

- Top-level `import_prefix` field on `Project` (`omitempty` — only the Infra framework seeds it;
  apps carry no CUE module).
- The Infra `Scaffold` seeds the placeholder `example.com` (a valid module path that renders
  immediately). The operator edits it — same empty-placeholder philosophy as the registry creds.
- Seeds the module path at init only. `cue.mod` is operator truth after generation and is never
  rewritten (`HasCueModule` guard), so changing `import_prefix` later means editing `cue.mod` too.

## Naming

`import_prefix` over the rejected candidates, each of which collides with a concept already live
in these webapp-deploy configs:

- `domain` — the webapp's **serving** domain (`#host:`).
- `namespace` — **k8s** namespaces, everywhere in the manifests.
- `module` / `mod_prefix` / `module_path` — platform's own `[modules]` build units.

`import` is unused elsewhere in platform and names exactly what the value drives.

## Deferred

Giving `import_prefix` teeth in the render pass — resolving the apps root as
`{import_prefix}/apps` rather than the hardcoded `./apps` — is backlog, not this ruling.
