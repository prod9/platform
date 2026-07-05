# Plan — Templated `init` + one-shot dogfood

Executable from a fresh session. Goal: **one `platform init` (or `ops init`) run produces a
fully working infra repo** — immediately ready to `render` → `build` → `publish` → `deploy`
into a working platform-managed cluster, **zero manual fixups**.

## Progress — 2026-07-06 (session 2)

Tasks 1–8 **done and validated end-to-end** (commits `5ce9e2b`, `eff663b`). A sandbox
`ops init` (temp dir, dummy creds) → `ops render` was zero-fixup: routing (`apps/`,
`defaults/basics.cue`, `cue.mod`, root), templating (creds into `defaults/basics.cue`,
`import "<module>/defaults"` + `dagger_version = "v0.21.7"` into `apps/platform.cue`), and
render all correct — `#Basics` emitted the `platform` namespace + `ghcr.io-pull-secret`, the
engine pinned `registry.dagger.io/engine:v0.21.7`, `imagePullSecrets` wired.

- Task 2 (`DefsModule` hardcoded) was already true — dropped.
- Concern boundary corrected: `baseline` owns infra-file copy/routing/templating
  (`baseline.Render`); `bootstrapper.AnalyzeInit` now produces only platform.toml + launcher
  + cue.mod; `ops init` stitches via `Plan.AddFile`.
- `defaults-basics.cue.tmpl` is `Mandatory` (always installed, never in the picker).
- Dagger version from linked SDK via `debug.ReadBuildInfo()` (`baseline.DaggerVersion`).

**Remaining before the real dogfood (task 9):**

1. **Flux self-sync fold** (parked `infra/apps/flux.cue`) — the `OCIRepository`+`Kustomization`
   that makes the cluster pull the config artifact (without it, deploy won't roll). **Open
   decision:** the `oci://` URL — committed literal vs. templated from `[ops].Image` at init.
   This is the last gap for a zero-fixup dogfood.
2. **Task 9 dogfood** — destructive (`rm -rf ./infra`), needs real creds + a live cluster;
   the operator's call to run.

## Settled design decisions — do NOT re-litigate

- **`defaults/` package is mandatory** on every infra repo. `apps/` holds **only render-able
  components** — every top-level key in `apps/` becomes an `ops render` output. Shared
  definitions (e.g. `#Basics`) live in `defaults/`, imported by `apps/`. This is the existing
  stage9 pattern, and platform init scaffolds repos onto it.
- **CUE `@tag` injection does not cross the module/package import barrier** (verified:
  `no tag for "X"` when the tag lives in an imported package). So registry creds **cannot** be
  `@tag`-injected into an imported `defaults/basics.cue`. We do **NOT** work around this by
  relocating the shared def into `apps/` — that breaks "apps is render-only". **The `@tag`
  approach is abandoned.** No workaround.
- **Creds (and the dagger version) enter via go-template at init time**, not CUE injection.
  `init` prompts for registry/username/password and renders `.tmpl` baseline files, writing
  **concrete** values into the target repo's `defaults/basics.cue`. Because the values are
  concrete after init, there is **no render-time concreteness check to build** (that idea is
  dropped — it couldn't cleanly isolate the cred error anyway).
- **`defaults/basics.cue` is required everywhere** → it ships in the baseline and is emitted on
  every init.

## Tasks (dependency order)

1. **Baseline naming = `folder-filename`.** Rename baseline source files so the destination is
   obvious from the name:
   - `baseline/files/apps-cert-manager.platform` → `apps/cert-manager.platform`
   - `baseline/files/apps-flux.platform` → `apps/flux.platform`
   - `baseline/files/defaults-basics.cue` (`.tmpl`) → `defaults/basics.cue`
   - `baseline/files/platform.toml.tmpl` → root `platform.toml`
   - (and `apps-platform.cue` / whatever the platform app template is named)
2. **`DefsModule` hardcoded** — it's not something to configure; hardcode it.
3. **`defaults/` package** — establish the `defaults/basics.cue` (imported by `apps/`) as the
   home for `#Basics`; keep `apps/` render-only.
4. **Dagger version source** — obtain the engine version from the **linked dagger module** (or
   read the modfile at `go:build`) instead of hardcoding it. It must match platform's linked
   `dagger.io/dagger` SDK, and it feeds the template.
5. **Templatize baseline** — convert `platform.cue` and the new `defaults/basics.cue` to
   go-templates (append `.tmpl`). Placeholders, go-template syntax:
   - dagger engine version (from task 4),
   - `#registry_username: "{REGISTRY_USERNAME}"`,
   - `#registry_password: "{REGISTRY_PASSWORD}"`,
   - `#registry` shipped **commented out** (defaults to `ghcr.io`).
6. **`ops.init` → `baseline`** — init should call into the `baseline` package to do the actual
   file-copying (it currently does not).
7. **Init prompts** — add a step during init to ask for registry, username, and password.
8. **Init template render** — amend the init routine to run a go-template render, injecting the
   dagger version (task 4) + prompted creds (task 7) into the emitted files.
9. **Dogfood = acceptance test.** Delete `./infra/`, run init **once**, and the result must be
   immediately ready to `render` → `build` → `publish` → `deploy` into a working
   platform-managed cluster with **zero manual fixups**. Any gap that needs hand-editing means
   the flow is incomplete — **revisit and fix/add features until a single init yields a fully
   working copy.**

## Current WIP to clean up first (uncommitted, from the abandoned `@tag` detour)

- `infra/apps/basics.cue` — the `@tag`-in-apps approach. **Delete** (wrong per the settled
  decisions).
- `infra/apps/flux.cue` — new this session. Its **content is the design to keep**: an
  `OCIRepository` (`oci://ghcr.io/prod9/infra`, tag `latest`, `secretRef: ghcr.io-pull-secret`)
  + a `Kustomization` (`sourceRef` → it, `path: ./`, `prune`, `wait`) + `#Basics`-provided
  namespace/secret. **Fold this into the baseline** (`apps-flux.*`), then it regenerates on
  init. Remove the duplicate `flux-system` Namespace from the flux-install directive so
  `#Basics` is the sole owner.
- `gitops/render.go` — the `missingTags` `@tag` warning + `sort`/`buildlog` imports I added.
  The `@tag` approach is abandoned; **decide whether to revert** (likely revert — it's moot).
- Rendered `k8s/flux/` files from test renders — regenerated, ignore/reset.

## Reference (this session's real, committed progress — mostly survives the infra blast)

- Delivery-verbs ADR: `docs/decisions/2026-07-05-delivery-verbs-are-orthogonal.md`
  (release/publish/deploy orthogonal; one publish engine, two drivers).
- `engine.BuildAndPublish` extraction + `ctx` reuse (`09d6a2a`, `842defe`).
- **gitops keychain fallback (`187f20d`) — KEEP.** `ops publish` now falls back to the docker
  keychain when `REGISTRY_USERNAME/PASSWORD` are unset. This is what let the local `ops publish`
  work; unrelated to the `@tag` detour.
- Dogfood shipped this session: `ghcr.io/prod9/platform:v0.8.4 @ sha256:383748c6` published;
  infra committed `10a4fd0` (image → v0.8.4, dagger engine v0.20.8 → v0.21.7). **This infra
  commit gets blown away by task 9's re-init — expected.**
- **Flux is now installed on stage9** (was absent — that's why v0.8.4 never rolled). The
  self-sync (`OCIRepository`+`Kustomization`) is the `flux.cue` design above; it still needs to
  be applied/bootstrapped once. See `docs/notes/2026-07-05-resume.md` "⚠️ Blocked — stage9 pull
  loop not wired".
- `packs.#Basics` (module `prodigy9.co/defs`, sibling repo `infra-defs/`) embeds a
  `[ns, secret]` list → callers need `[...]` at the use site. Stage9 idiom; not ours to
  "fix" here.
- stage9 reference for the pattern: `~/Documents/prod9/infra-stage9/` (`defaults/basics.cue`
  pins creds as literals; every `apps/*.cue` does `defaults.#Basics & {#name, [...]}`).

## Pending school change (for `ace-school` to propose later)

- **cue-coding skill**: record that CUE `@tag`/`-t` injection is **root-package only** — it does
  NOT cross the module/package import barrier (imported packages error `no tag for "X"`).
  Verified this session. Corollary: values needed inside an imported package must arrive by a
  non-`@tag` route (literal, or upstream go-template render), not tag injection.
