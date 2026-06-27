# platformv2 — Spec

Consolidates CUE manifest-gen, the infra CLI, this build tool, and the keel+argo glue into
**one** tool: a GitHub-centric, in-cluster build + deploy **control plane** for PRODIGY9.
Companion docs: [`config-allocation.md`](config-allocation.md) (who-owns-what),
[`architecture.md`](architecture.md) (the build pipeline + object model), and the ADRs in
[`../decisions/`](../decisions/).

## What it is

A server, run inside the (single, v2) home cluster, that owns the way-of-work: identity
and access, Projects, image builds, gated deploys, secret brokering, and cluster
bootstrap. It is **API-first** — the UI, CLI, and an OpenTofu provider are three clients
of one API. It never pushes into the cluster: delivery is **pull-based** (Flux), and
platform acts locally through its pod ServiceAccount.

The reason it's a server and not a CLI: **identity/RBAC**. Self-serve project setup,
prune-on-leave, and killing the scattered root credentials are the justification — build
and deploy hang off that.

## Way of work

End-to-end, verbs and artifacts:

1. **Onboard.** A source repo gets a `platform.toml` (build metadata + infra pointer +
   Project binding) and is bound to a **Project** in platform (UI/CLI/tf-provider). Access
   is members + roles on the Project.
2. **Authenticate.** A person logs in via GitHub; platform links their GitHub identity to
   an internal user and issues its **own** token. Their role on the Project decides what
   they can reach.
3. **Build.** A code push builds the app image (Dagger) → an **immutable** tag in the
   registry. (CI reflex; also runnable via `platform build`.)
4. **Deploy (gated).** An authorized user promotes a built image to a **target**: platform
   writes the immutable ref into that target's `infra/` CUE, **author-as-user** via the
   GitHub App. The commit is the gate.
5. **Render + publish.** Platform (CI) renders the `infra/` CUE via the **linked CUE engine**
   (`cuelang.org/go`, in-process — no `cue` binary) over infra-defs, and pushes the **rendered
   manifests** as the config OCI artifact under the target's **moving** tag. Third-party
   installs (cert-manager, NGF) are adapted by the
   [manifest patch DSL](manifest-patch-dsl.md), not CUE.
6. **Reconcile.** Flux follows the moving tag → applies/prunes → pods run the pinned
   image. Drift is corrected continuously.
7. **Secrets.** Each pod's init-container pulls its secrets from platform (outbound) at
   start; values stay in platform otherwise.
8. **Operate.** Devs reach the cluster with `kubectl` via a platform-minted, RBAC-scoped,
   short-lived SA token (exec-credential plugin), and see target status in the UI (Flux CR
   status).
9. **Bootstrap.** Targets/envs are declared in `tf/` (OpenTofu, manual local apply in v2).
   Platform writes the operator-chosen subset of its embedded baseline into the infra repo; a
   new cluster is seeded once (manual: Flux + that baseline), then Flux reconciles the rest.

No credential reaches into the cluster — the cluster pulls everything.

## Component contracts

- **`api/` — platform server.** In-cluster, pod SA, Postgres. Owns Projects,
  identity/RBAC, secret *values*, audit, deploy history. Brokers: kube tokens
  (`TokenRequest`), the secret-pull endpoint, the gated deploy (git dance). Serves the API
  the other clients use.
- **`cli/` — platform CLI** (+ folded OpenTofu provider as a multi-call binary). `login`
  (device-flow GitHub OAuth → platform token), `build` /`preview` (local Dagger, no
  deploy), `deploy` (API call), `kubeconfig` (exec-credential), `tf install`.
- **`ui/` — SvelteKit (plain JS)**, adapter-static, `go:embed` 'd into `api`. v1: Login,
  Projects, Access, Deploys, Target status.
- **Shared Go packages** — flat at the top level, no `core/` grab-bag (see
  [`architecture.md`](architecture.md)): `builder` (Dagger strategies), `engine` (the Dagger
  pool + executor), `project` (`platform.toml`), `releases`, `gitctx`, `gitops`
  (linked-CUE-engine render + OCI publish), `dsl`, `baseline`, `ops`; api-client + shared
  types land as the server grows.
- **Flux** — source-controller + kustomize-controller. Reconciles config artifacts;
  prunes; corrects drift. Its own lifecycle is *not* self-managed. No Argo, no Helm.
- **Dagger engine** — in-cluster; builds run inside the engine pod (engine-opaque); the
  engine pod is the resource-managed unit, sized like any workload.
- **Registry (OCI)** — app images (immutable) + config artifacts (moving per-env tag).
- **Postgres** — platform state (projects, `users` /`identities`, secrets-encrypted,
  audit).
- **`platform-init`** — the baseline (Flux, cert-manager, NGF, engine, platform),
  **embedded in the tool** as a flat list of `.cue` apps + `.platform` directives; `ops init`
  writes the operator-chosen subset into the infra repo's `apps/`, seeded once then
  Flux-reconciled. Not a separate repo — see the
  [appliance ADR](../decisions/2026-06-17-opinionated-appliance-embedded-init.md) and the
  [flat-baseline ADR](../decisions/2026-06-22-flat-baseline-install-time-selection.md).

## Server scope

A real control plane, justified by identity/RBAC consolidation (not a minimal broker). It
owns identity, projects, access, secrets, build orchestration, deploy gating, and the
kube-token + secret brokering. It **triggers/feeds** the reconcilers (via git + OCI) and
**never reconciles** in-cluster state itself — pull-only, no inbound cluster creds.

## Identity & access

- Internal **`users`** are the anchor; **`identities`** rows link external accounts
  (`provider`, stable `provider_id`, `metadata` jsonb, `kind` login|service). GitHub is
  the only adapter in v2; Google/Sentry/custom slot in later with zero schema change.
- Auth providers are pluggable behind an `IdentityProvider` interface; a generic OIDC
  adapter later covers most. Authz is keyed on internal users + a claim→role mapping
  (GitHub teams → roles now) — never hardcoded to GitHub.
- Platform issues its own session token; all downstream (CLI, kube-broker, tf-provider)
  consume platform identity.
- Same verified email across trusted providers auto-links to one user; per-provider
  `trust`
  + `email_verified` gate it (no auto-merge on unverified/untrusted).

## Phase boundaries

- **v2** — single home cluster; platform in-cluster; GitHub-only IdP; secrets via
  platform-pull init-container; `tf/` manual; no DNS.
- **v2.1** — DNS (Cloudflare via `tf/`), PR/branch deploys, the approvals/plan-gate UI,
  platform-run tofu.
- **phase 2** — multi-cluster (central control-plane + per-cluster agents); more
  IdPs/service links.

## Anchors

- **Opinionated appliance.** Platform ships *the* setup (Flux + cert-manager + NGF +
  engine + a specific Gateway topology) and does not work against any other. The cluster
  baseline is platform's opinion, embedded and version-locked with the tool — not external
  operator config. See the
  [appliance ADR](../decisions/2026-06-17-opinionated-appliance-embedded-init.md).
- **Language: Go** (+ CUE via the linked engine, not the `cue` binary; UI in SvelteKit/JS —
  no TypeScript, no Helm, no timoni — see
  [renderer ADR](../decisions/2026-06-16-renderer-cue-export-not-timoni.md)).
- **Trigger reconcilers, don't be one.** Pull-based; platform feeds Flux, never reconciles
  in-cluster.
- **No inbound cluster creds.** The load-bearing invariant; it shapes every surface.
- **Sequencing, not big-bang.** Consolidate along the spine (build → render → publish →
  reconcile); migrate the monorepo and fold-ins incrementally.

## Open questions

- Monorepo migration sequencing (touches test harness, Dockerfile, bootstrapper).
- v2.1 / phase-2 scope detail (DNS, branch deploys, multi-cluster agent protocol).
