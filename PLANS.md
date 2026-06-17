# Pending Work — Plans for Approval

**Primary direction (2026-06-14): platformv2 — server rewrite.** Design walk complete and
committed (`0742a03`). The legacy numbered plans (#3–#7) are pre-existing build-pipeline
tasks: #3 is superseded by v2; #4/#5/#7 carry forward into the v2 build pipeline.

## platformv2 — server rewrite

Design settled in `docs/spec/` (`platform.md`, `config-allocation.md`,
`gitops-build-plan.md`) + ADRs in `docs/decisions/`. In one line: a GitHub-centric,
in-cluster control plane delivering via pull-based GitOps (`cue export` + Flux + OCI),
retiring BuildKite/Keel/Argo; **no inbound cluster creds**; platform runs in-cluster via
its pod SA. (Renderer is `cue export`, not timoni — see the renderer ADR.)

**Implementation plan** → `docs/notes/2026-06-16-platformv2-implementation-plan.md` (the
living roadmap): phased (A spine → **A′ patch DSL + embedded init** → B control plane → C
fold-ins). **Slice 1 (render + publish) landed 2026-06-17.** The DSL was pulled forward to
A′ (it is the primitive the embedded baseline depends on — see the
[appliance ADR](docs/decisions/2026-06-17-opinionated-appliance-embedded-init.md)).
Spine-first, not big-bang. Build backlog (now folded into those phases):

- **Monorepo restructure** — `api/` `cli/` `ui/` `core/`, one Go module; SvelteKit-**JS**
  `ui/` via adapter-static `go:embed` 'd into `api`. Touches test.sh/tests.cue/testbeds,
  Dockerfile, bootstrapper — migrate incrementally.
- **OpenTofu provider = the CLI binary** (multi-call; `terraform-provider-platform`
  symlink → gRPC provider; `platform tf install` writes `dev_overrides`).
- **platform server (`api/`)** — fx + Postgres; Projects, identity (`users`/`identities`,
  GitHub adapter, pluggable), RBAC, audit, deploy history.
- **Delivery** — `cue export` (over infra-defs) → multi-doc YAML → rendered OCI artifact
  (moving per-env tag) → Flux (`OCIRepository` + kustomize-controller); app images
  immutable. Third-party installs adapted by the manifest patch DSL.
- **Brokers** — kube token (`TokenRequest`, pod SA), secret-pull init-container, gated
  deploy (git dance, author-as-user via GitHub App).
- **UI v1** — Login, Projects, Access, Deploys, Target status (Flux CR status).
- **infra-cli generators** (cert-manager/nginx-gateway/generate) → fold into the
  **embedded** `platform-init` baseline as an **init DSL package**, NOT CLI subcommands
  (supersedes #3); the `argocd` generator → Flux. The DSL backend is a port of
  `infra-cli/pipelines/*` + `yamleditor` — see Phase A′ in the roadmap.

Fold the legacy tasks into v2: **#7 version injection** (build provenance — design into
the v2 build pipeline), **#4 container hardening** (apply to v2 runner images), **#5
plog→fxlog** (still applies; needs the fx bump v2 wants anyway).

---

## Status (legacy plans)

Approve one at a time; each lands as its own commit sequence.

- **#3 infra-cli fold-in** — ❌ SUPERSEDED by platformv2 (infra-cli's generators fold into
  the `platform-init` baseline, not CLI subcommands; see above).
- **#4 Privilege drops** — pending.
- **#5 plog → fxlog** — pending; gated on fx bump in #3.
- **#6 Wolfi pin** — ✅ done (digest pin + cache buster bump).
- **#7 Version injection** — pending; build provenance for deployed images.

## Resume hint

**Plan locked, decisions taken** —
`docs/notes/2026-06-16-platformv2-implementation-plan.md` (living roadmap). timoni dropped
(`7a9e13b`): renderer = `cue export` over infra-defs + a Go multi-doc emit; foreign
installs patched by the manifest patch DSL (`docs/spec/manifest-patch-dsl.md`). Platform
is an **opinionated appliance** — the cluster baseline is embedded and shipped as an init
DSL package (appliance ADR `2026-06-17`).

**Slice 1 — DONE.** Render half (`615caa4`): `core/gitops.Render` shells
`cue export -e objects --out yaml`, splits via `yaml.Node` walk. Publish half (`c9ffc0c`):
`core/gitops.Publish` packs manifests as a Flux-shaped OCI artifact via oras-go;
`core/gitops.RemoteRepository` does `oci://` + `REGISTRY_USERNAME/PASSWORD` auth. Both
under the `platform ops` namespace (`ops render` / `ops publish`). Unit round-trip via
`oci.Store`; no live-registry smoke. Docs realigned (`20ca12a` + the 2026-06-17 alignment
pass).

Next: **Phase A′ — Slice D1 (DSL core)**, the hermetic patch engine: port
`infra-cli/pipelines/yamleditor` (path-walk + new `[name=v]` field-select) + the in-buffer
verbs + the directive parser, into `core/patch`. Unit-tested on inline fixtures, no
network. Detail + acceptance criteria in the roadmap's "Slice D1" section. Then D2 (I/O
verbs + `emit`), D3 (init DSL package + bootstrap-writes-DSL, dogfooded against the real
`infra` repo), then Slice 2 (reconcile + cutover). Legacy #4/#5/#7 fold into Phase B/C —
detail below.

Note: re-running `./test.sh -c` rewrites the whole lock with single-quote YAML (emitter
drift vs the committed double-quoted entries) — harmless (smoke compares parsed values), but
to avoid churn, add new lock entries surgically rather than full `-c`.

Session state as of 2026-06-14:
- platformv2 design walk complete; docs committed `0742a03` (spec + config map + 5 ADRs).
- Uncommitted, left for you: `docs/chakrit-reply.md` and `PLANS.md` (this file); plus a
  Helm-ban note added to your dotfiles `~/.claude/CLAUDE.md` (separate repo, uncommitted).

Earlier session state (2026-06-12):
- #6 landed in `64ae2b3`; smoke suite green (note: 1m timeout in tests.cue is deliberately
  tight — fix slowness, never raise it; cold-pull verified by warming cache).

---

## 3. Fold `prod9/infra-cli` into `./platform infra ...`

**Source repo:** `github.com/prod9/infra-cli`, module `infra.prodigy9.co`, Go 1.25.5.

**Subcommands to absorb** (from `cmd/`):
- `argocd` (`argocd_cmd.go`, 2.7K)
- `cert-manager` (`cert_manager_cmd.go`, 2.6K)
- `generate-aoa` (`generate_aoa_cmd.go`, 1.3K)
- `generate` (`generate_cmd.go`, 5.0K) — the main one
- `init` (`init_cmd.go`, 0.4K)
- `nginx-gateway` (`nginx_gateway_cmd.go`, 6.3K, has tests)
- `settings` (`settings_cmd.go`, 0.4K)
- `vanity` (`vanity_cmd.go`, 1.1K) — **conflicts with platform's existing `vanity` cmd**
  (different vanity host).

**Auxiliary trees:** `pipelines/`, `settings/`, `templates/` (each non-empty directory at
repo root). Need to copy alongside `cmd/`.

**Dependency deltas to reconcile:**
| Dep                   | platform    | infra-cli  | Action                                                                                                         |
| --------------------- | ----------- | ---------- | -------------------------------------------------------------------------------------------------------------- |
| `fx.prodigy9.co`      | `v0.4.0`    | `v0.8.2`   | Bump platform to `v0.8.2`, fix any API drift. Likely affects `cmd/prompts`, `cmd/ctrlc`, `cmd/PrintConfigCmd`. |
| `cuelang.org/go`      | (test only) | `v0.15.4`  | Add as direct dep.                                                                                             |
| `BurntSushi/toml`     | `v1.4.0`    | `v1.6.0`   | Bump.                                                                                                          |
| `spf13/cobra`         | `v1.8.1`    | `v1.10.2`  | Bump.                                                                                                          |
| `pterm/pterm`         | `v0.12.79`  | `v0.12.82` | Bump.                                                                                                          |
| `go.jonnrb.io/vanity` | `v0.2.0`    | `v0.2.0`   | OK.                                                                                                            |
| `gopkg.in/yaml.v3`    | indirect    | direct     | Promote.                                                                                                       |

**Plan:**
1. **Pre-work commit** (separate): bump shared deps in platform alone (`fx.prodigy9.co`,
   `cobra`, `toml`, `pterm`) and fix compile errors. This isolates dep-bump churn from the
   fold-in.
2. Create `cmd/infra/` package in platform mirroring infra-cli's `cmd/` layout. Copy each
   `*_cmd.go`, rename root binding from `infra <sub>` to nested cobra group `infraGroup`.
3. Add `var InfraCmd = &cobra.Command{Use: "infra", Short: "..."}` exposed from
   `cmd/infra/`. Wire children: `argocd`, `cert-manager`, `generate`, `generate-aoa`,
   `init`, `nginx-gateway`, `settings` under it. Skip `vanity_cmd.go` — keep platform's
   existing vanity, or namespace infra-cli's as `infra vanity` if the host differs.
4. Copy `pipelines/`, `settings/`, `templates/` into `cmd/infra/` (or a new `infraassets/`)
   and `//go:embed` them as needed. Update import paths in copied files from
   `infra.prodigy9.co/...` to `platform.prodigy9.co/cmd/infra/...`.
5. Wire `cmd.InfraCmd` into `main.go` `rootCmd.AddCommand(...)` block.
6. Port the test (`nginx_gateway_cmd_test.go`).
7. Verify with `go build ./... && go test ./...` and a manual smoke of one
   `./platform infra generate ...` command.
8. After merge: open PR on `prod9/infra-cli` archiving the repo with a pointer to
   platform.

**Risks:**
- `fx.prodigy9.co` v0.4 → v0.8 is a four-minor bump; expect breakage in `prompts.New`,
  `cmd.PrintConfigCmd`, `errutil`, and `ctrlc`.
- `vanity` collision needs a decision before step 3.
- Embedded templates may reference relative paths that change once moved.

**Out of scope:** rewriting any infra-cli logic; this is a verbatim fold-in.

---

## 4. Drop privileges / harden built containers

**Current state:** `BaseImageForJob` runs `apk update/upgrade` as root and sets
`WithWorkdir("/app")` but never sets a non-root user, capabilities, read-only rootfs, or
healthcheck. Containers ship running as `root` with default Wolfi caps.

**Simplest hardening (high impact / low effort), in order:**

### 4a. Run as non-root (`USER nonroot`)
Wolfi already ships a `nonroot` user (uid 65532). Add to `BaseImageForJob` runner path
**only** (build path stays root so apk/build steps work):

```go
runner = runner.
    WithExec([]string{"chown", "-R", "65532:65532", "/app"}).
    WithUser("nonroot")
```

Apply in `withRunnerPkgs` or a new `withNonRootUser` helper called in each builder's
runner stage (go_basic, go_workspace, pnpm_basic, pnpm_workspace, pnpm_static).

**Watch-outs:**
- Caddy in `pnpm/static` listens on `:3000` (already non-privileged, fine).
- Anything binding `<1024` would break — none of our builders do.
- `WithDefaultArgs` runs as the configured user; verify `/app` ownership.

### 4b. Drop unused capabilities at runtime
Document in `bootstrapper/buildkite.pipeline.yaml.template` (and any K8s manifests
downstream) the recommended pod `securityContext`:
```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 65532
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities: { drop: ["ALL"] }
seccompProfile: { type: RuntimeDefault }
```
Image side can't enforce this — runtime must. Provide as deploy-time guidance.

### 4c. Read-only rootfs hint
At image build time, declare `/tmp` and any writable scratch dirs as `VOLUME` so
`readOnlyRootFilesystem: true` deployments still work. Use
`Container.WithMountedTemp("/tmp")` in the runner stage if Dagger exposes that primitive
in 0.20.

### 4d. Tini / signal handling (optional)
Wolfi has `tini`. Adding `withRunnerPkgs(..., "tini")` and prepending `/usr/bin/tini`,
`--` to default args fixes `Ctrl-C` propagation in single-process containers. Low risk.

**Plan:**
1. Add `withNonRoot(runner, "/app")` helper in `builder/base.go`.
2. Call it from each builder's runner stage (5 sites). Verify testbeds.
3. Update `bootstrapper/buildkite.pipeline.yaml.template` (if it produces K8s manifests;
   if not, add docs to `README.md`).
4. (Optional) tini integration as a follow-up commit.

**Risks:** breaking existing deployments that mount writable volumes or expect to write in
`/app`. Document the change loudly in the release notes.

**Out of scope:** distroless migration, image signing (cosign), SBOM generation — separate
hardening tracks.

---

## 5. Replace `internal/plog` with `fxlog`

**Depends on:** task #3 (`fx.prodigy9.co` v0.4.0 → v0.8.x bump). `fxlog` is not in v0.4.0;
it appears in v0.5+.

**Surface:** `internal/plog/{plog,events}.go` exposes `Fatalln`, `Error`, `Event`,
`Config`, `Git`, `GitInfo`, `File`, `Image`, `HTTPServing`, `HTTPRequest`,
`OutputForDagger`, `SetVerbosity`, `Logger`. Roughly 40 call sites across `cmd/`,
`builder/`, `releases/`, `bootstrapper/`, `gitctx/`, and `main.go`.

**Plan:**
1. After the fx bump lands, inspect `fxlog` 's public API in the upgraded module cache.
   Confirm equivalents for each plog function — particularly:
   - `OutputForDagger() io.Writer` — used by `builder/session.go` for
     `dagger.WithLogOutput`. fxlog must expose a writer or a slog handler we can bridge.
   - `SetVerbosity(int)` — wired to `-q` /`-v` in `main.go`. Map to fxlog's level control
     (zerolog level by default per fx skill notes).
2. Migrate call sites in one sweep. Prefer parallel `Edit` calls per package.
3. Delete `internal/plog/` once empty. If fxlog lacks 1–2 helpers, keep a thin wrapper
   file (≤30 lines) rather than spreading inline boilerplate.
4. `go build ./... && ./test.sh`.

**Risks:** log format change may break stdout assertions in `tests.cue` testbeds. Audit
and update expectations.

**Out of scope:** changing which events get logged or at what level.

---

## 6. Pin and refresh Wolfi base image

**Current:** `BaseImageName = "cgr.dev/chainguard/wolfi-base"` (untagged, resolves to
`:latest`). No pinning, no digest, no refresh cadence.

**Plan:**
1. Resolve current digest:
   `docker buildx imagetools inspect cgr.dev/chainguard/wolfi-base:latest`. Record the
   `sha256:...` digest and any human-readable tag.
2. Pin via digest in `builder/base.go`:
   `BaseImageName = "cgr.dev/chainguard/wolfi-base@sha256:..."` with the readable tag in
   an adjacent comment. Digest > tag because Chainguard's `:latest` is a floating ref;
   reproducibility wins over readability here, and the comment covers the readability gap.
3. Bump `CacheBuster` to force re-pull across environments.
4. Verify Dagger pulls the digest-pinned ref (`From` accepts `@sha256:`; no API change
   needed).
5. Run `./test.sh` across all six testbeds.
6. Document a manual refresh cadence (e.g. monthly) in the `builder/base.go` package doc
   added during the Wolfi audit. No automation this round.

**Risks:** Pinned digest ages → base-layer CVEs require manual bump. `apk update/upgrade`
in `BaseImageForJob` keeps userland packages fresh, so drift is limited to the base layer
itself. Cadence doc sets the expectation.

**Out of scope:** automated digest-bump bot, cosign signature verification, SBOM.

---

## 7. Inject build version metadata into runner images

**Problem:** apps want a `/versionz` endpoint proving which commit a deployed pod runs
(image tags are mutable). The naive hack removes `".git"` from `excludes` so `go build`
inside Dagger stamps VCS info via `debug.ReadBuildInfo()` — ships the whole `.git` dir
into every build context (weight + cache churn) and every downstream repo would have to
repeat it.

**Design: add a metadata pass to the build pipeline.** Today builds are effectively two
passes (builder stage → runner stage) per module. Insert a **metadata pass before them** —
runs once per build invocation on the host (not per module, not inside Dagger):

1. **Metadata pass** — collect from the host where `.git` exists, via `gitctx` /`gitcmd`
   (builders don't use them yet): commit sha, dirty flag, current release tag (from
   `releases/`), build timestamp. Produce a `BuildMeta` struct handed to every job.
2. **Builder pass** — unchanged. Metadata must NOT enter this stage, so compile caches
   never invalidate on sha change.
3. **Runner pass** — each builder's runner assembly (go_basic, go_workspace, pnpm_basic,
   pnpm_static, pnpm_workspace, dockerfile) injects, as late as possible:
   - `/app/version.json` —
     `{"commit": "...", "dirty": false, "release": "v20260605-2", "built_at": "..."}`
     (file form suits static/pnpm apps).
   - `PLATFORM_COMMIT` env var (cheap, suits Go apps). Possibly `PLATFORM_RELEASE` too.

**Notes:**
- `WithNewFile` /`WithEnvVariable` land after build steps → no cache impact.
- Timestamp source must respect reproducibility (commit time, not wall clock, is a
  candidate — decide at planning).
- OCI labels (`org.opencontainers.image.revision`) on the runner image are a near-free
  addition while we're here — registry-side traceability.
- Dockerfile builder: env/label injection still possible post-build even though the FROM
  is user-controlled.

**Out of scope:** serving `/versionz` itself (app concern), image signing.

---

## Suggested execution order

1. **#7 (Version injection)** — small-mid, standalone, no dependencies.
2. **#4 (Privilege drops)** — independent; verify Dagger 0.20 primitives during
   implementation. Touches the same runner-stage sites as #7 — landing #7 first gives #4
   the metadata-pass plumbing to build on.
3. **#3 (infra-cli fold-in)** — biggest blast radius. Requires the `fx.prodigy9.co` bump
   which is its own mini-project.
4. **#5 (plog → fxlog)** — gated on the fx bump from #3. Land right after #3 while the API
   is fresh in mind.

Approve plans individually and I'll execute one at a time.
