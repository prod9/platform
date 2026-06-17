# Manifest Patch DSL

**Status:** accepted (design 2026-06-16) · **next up** — pulled forward from Phase C
(2026-06-17), it is the primitive the embedded baseline / init package depends on.
**Decided in:** [renderer ADR](../decisions/2026-06-16-renderer-cue-export-not-timoni.md),
[appliance ADR](../decisions/2026-06-17-opinionated-appliance-embedded-init.md). Build plan:
[roadmap](../notes/2026-06-16-platformv2-implementation-plan.md) Phase A′.

A line-oriented directive language for adapting third-party Kubernetes manifests we don't
own (cert-manager, NGINX Gateway Fabric, …) — fetch upstream, patch by name, emit. CUE
handles manifests we author; this handles foreign ones. Folded from infra-cli's
`pipelines` + `pipelines/yamleditor` (~676 LOC incl. tests; the verbs already exist as Go
pipeline ops — only the directive parser and the field-select path form are new code). Its
first consumer is the **init DSL package** (the embedded cluster baseline), dogfooded
against the real `infra` repo (`apps/cert-manager.cue`, `k8s/nginx-gateway`, …).

## Why a closed vocabulary, not a script

A general-purpose embedded language (Lua, Starlark, CEL, yq-expr) can't be bounded by
reading it — the reviewer must execute it mentally. A fixed set of verbs can: a directive
file is fully understood top to bottom, no hidden control flow. That readability bound is
the whole point, and the same reason Helm and TypeScript are banned here.

## Model

A directive file is a sequence of lines applied to a multi-document YAML stream held in a
buffer. `select` sets the active document scope; subsequent edit verbs apply to every
document in scope, at the path each names. Scope holds until the next `select`. The
back-end is the existing `yamleditor` path-walk (`Get`/`Set` over `map[string]any` /
`[]any`); the DSL is a thin front-end over it.

Branch-free by design: no conditionals, no loops. Config-gating (include this edit only
when DaemonSet mode is on) happens at *assembly* time — the layer emitting the directive
list decides which lines to include. `${var}` substitution supplies values; substitution
only, no expressions.

**Where `${var}` values come from (proposed, see open-questions #10):** the infra repo's
`settings.toml` — already a typed, per-component store (`settings.Settings`:
`[cert_manager].version`, `[nginx_gateway].{version,gateway_api_version,experimental,…}`).
The per-component assembly layer reads it, maps versions → the `${var}` map, and gates
which directive lines to emit on the bools (`experimental`/`daemonset`). `platform.toml`'s
`[ops]` stays scoped to the publish target, not versions. An env-var override over
settings (e.g. `NGINX_GATEWAY_VERSION=…`) is a candidate convenience, not the primary
store.

## Path grammar

Dotted keys with two index forms:

- `.spec.replicas` — map keys.
- `.spec.containers[0]` — numeric list index.
- `.spec.containers[name=cert-manager-controller]` — field-select: the list element whose
  `name` equals the value. This is the load-bearing form — upstream reorders containers
  between versions, so index targeting is a latent bug.

## Verbs

| Verb                   | Effect                                                       |
| ---------------------- | ------------------------------------------------------------ |
| `download URL`         | fetch into the buffer                                        |
| `extract-zip P`        | replace the buffer with file `P` from a zip in the buffer    |
| `select K=V …`         | scope to docs matching all `key=value` (`kind=`, `name=`, …) |
| `set PATH V`           | set scalar at PATH, creating intermediates                   |
| `set-if-absent PATH V` | set only when PATH is unset (idempotent guard)               |
| `append PATH V`        | append V to the list at PATH, creating it if absent          |
| `append-unique PATH V` | append only when V is not already present (idempotent)       |
| `delete PATH`          | remove the field at PATH                                     |
| `delete-doc`           | drop every document in scope from the stream                 |
| `emit`                 | hand the buffer to the render/publish pipeline               |

Changing kind is just `set .kind DaemonSet` — `.kind` is a path like any other; no
dedicated verb.

## Examples

cert-manager — append controller flags to a named container, version-robust, idempotent:

```
download https://github.com/cert-manager/cert-manager/releases/download/${version}/cert-manager.yaml
select  kind=Deployment name=cert-manager
append-unique .spec.template.spec.containers[name=cert-manager-controller].args --enable-gateway-api
append-unique .spec.template.spec.containers[name=cert-manager-controller].args --feature-gates=ListenerSets=true
```

NGF → DaemonSet — set a field, delete siblings:

```
select kind=Deployment name=nginx-gateway
set    .kind DaemonSet
delete .spec.replicas
delete .spec.strategy
```

NGF serverTokens workaround / argo doc-drop:

```
select        kind=NginxProxy namespace=nginx-gateway
set-if-absent .spec.serverTokens off

select     kind=Secret name=argocd-secret
delete-doc
```

## Notes

- **Idempotency matters** — directives re-run on every upstream version bump, so
  `append-unique` / `set-if-absent` are first-class, not conveniences.
- **v2 tail** — infra-cli ended pipelines with `write` + `kubectl apply` + `git commit`.
  In v2 Flux applies, so the tail is `emit` into the publish artifact; `kubectl` / `git`
  drop out. The `download → patch` head is unchanged.
- **Back-end reuse** — `yamleditor.Get` /`Set` already do path-walk with int-index list
  access and create-if-absent; the field-select form and the verb parser are the only new
  code.
