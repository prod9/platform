# platformv2 — Spec

Status: **draft / vision.** The north-star shape of the control plane, not a description of
what ships today — read it for intent and direction. Current built behavior lives in the
sibling specs ([`architecture.md`](architecture.md), [`frameworks.md`](frameworks.md),
[`engine.md`](engine.md), [`releases.md`](releases.md)); where this file and those disagree,
they win.

Consolidates CUE manifest-gen, the infra CLI, this build tool, and the keel+argo glue into
**one** tool: a GitHub-centric, in-cluster build + delivery **control plane** for PRODIGY9.
Companion docs: [`config-allocation.md`](config-allocation.md) (who-owns-what),
[`architecture.md`](architecture.md) (the build pipeline + object model), and the ADRs in
[`../decisions/`](../decisions/).

## What it is

A server, run inside the (single, v2) home cluster, that owns the way-of-work: identity
and access, Projects, image builds, gated desired-state commits, secret brokering, and cluster
bootstrap. It is **API-first** — the UI, CLI, and an OpenTofu provider are three clients
of one API. It never pushes into the cluster: delivery is **pull-based** (Flux), and
platform acts locally through its pod ServiceAccount.

The reason it's a server and not a CLI: **identity and credential brokering**. Self-serve
project setup, prune-on-leave, and killing the scattered root credentials are the
justification — build and delivery hang off that. Authorization is *not* among them: it
stays GitHub's, and platform holds zero RBAC of its own.

## Way of work

End-to-end, verbs and artifacts:

1. **Onboard.** A source repo gets a `platform.toml` (build metadata + infra pointer +
   Project binding) and is bound to a **Project** in platform (UI/CLI/tf-provider). Access
   follows the repo — platform defines no access of its own.
2. **Authenticate.** A person logs in via GitHub; platform links their GitHub identity to
   an internal user and issues its **own** token. What they can reach is what GitHub says
   they can reach: repo access triggers builds, infra-repo push permission deploys.
3. **Build.** A code push builds the app image (Dagger) → an **immutable** tag in the
   registry. (CI reflex; also runnable via `platform build`.)
4. **Commit the new ref (gated).** An authorized user changes the app-image ref in the app's `infra/` CUE —
   hand-edited and committed, or (later) the server authoring that commit **as the user** via the
   GitHub App. The gate is the user's GitHub push permission on the infra repo; the commit is the
   record. Platform never rewrites the operator's CUE. There is no `deploy` verb.
5. **Render + publish.** Infra is a framework: it renders the `infra/` CUE via the **linked
   CUE evaluator** (`cuelang.org/go`, in-process — no `cue` binary) over infra-defs, packs the
   **rendered manifests** into a `FROM scratch` image, and `publish` pushes it under a
   **moving** tag — the ordinary Dagger publish path, no bespoke OCI pusher (see
   [infra-publishes-as-plain-image-retire-oras](../decisions/2026-07-05-infra-publishes-as-plain-image-retire-oras.md)).
   Third-party installs (cert-manager, NGF) are adapted by the
   [manifest patch DSL](manifest-patch-dsl.md), not CUE.
6. **Reconcile.** Flux follows the moving tag → applies/prunes → pods run the pinned
   image. Drift is corrected continuously.
7. **Secrets.** Each pod's init-container pulls its secrets from platform (outbound) at
   start; values stay in platform otherwise.
8. **Operate.** Devs reach the cluster with `kubectl` via a platform-minted, RBAC-scoped,
   short-lived SA token (exec-credential plugin), and see reconcile status in the UI (Flux CR
   status).
9. **Bootstrap.** Cloud resources come from `tf/` (OpenTofu, manual local apply in v2);
   multi-env is infra-repo CUE + namespacing, not a platform target list. The `Infra` framework
   writes its full embedded baseline into the infra repo; a new cluster is seeded
   once (manual: Flux + that baseline), then Flux reconciles the rest.

No credential reaches into the cluster — the cluster pulls everything.

## Subsystem contracts

- **`srv/` — platform server** (reached via `platform serve`; future). In-cluster, pod SA,
  Postgres. Owns Projects, identity, secret *values*, audit, delivery history. Brokers: kube
  tokens (`TokenRequest`), the secret-pull endpoint, the commit-as-user git dance (gated by
  GitHub push perms). Serves the
  API the other clients use. Authorization is **GitHub-derived, zero platform RBAC** — see
  [platform-server-github-app-zero-rbac](../decisions/2026-06-29-platform-server-github-app-zero-rbac.md).
- **`platform` CLI** (the existing `cmd/`-based binary; + folded OpenTofu provider as a
  multi-call binary). `login` (GitHub OAuth → platform token), `build`/`preview` (local
  Dagger), `kubeconfig` (exec-credential), `tf install`. (No `deploy` command — a new version is
  an infra-repo commit; delivery is `render` + `publish`, infra being a framework.)
- **`webui/` — SvelteKit (plain JS)**, adapter-static, `go:embed`'d into the `srv/` binary.
  v1: Login, Projects, Access, delivery history, reconcile status.
- **Shared Go packages** — flat at the top level, no `core/` grab-bag (see
  [`architecture.md`](architecture.md)): `framework` (frameworks — discover/scaffold/build
  strategies) with `framework/scaffold` (the one templating mechanism), `engine` (the Dagger
  runtime + executor), `conf` (`platform.toml`, incl. the top-level `[vars]` table),
  `releases`, `git`, `gitops` (linked-CUE-engine render, with `gitops/dsl`); api-client + shared types
  land as the server grows.
- **Flux** — source-controller + kustomize-controller. Reconciles config artifacts;
  prunes; corrects drift. Its own lifecycle is *not* self-managed. No Argo, no Helm.
- **Dagger engine** — in-cluster; builds run inside the engine pod (engine-opaque); the
  engine pod is the resource-managed unit, sized like any workload.
- **Registry (OCI)** — app images (digest-pinned) + config artifacts (moving tag; git = record).
- **Postgres** — platform state (projects, `users` /`identities`, secrets-encrypted,
  audit).
- **`platform-init`** — the cluster baseline (Flux, cert-manager, NGF, engine, platform),
  embedded in the tool rather than a separate repo, installed unconditionally by the `Infra`
  framework and seeded once, then Flux-reconciled. The file set and its destination-encoding
  rules are spec'd in [`scaffolding.md`](scaffolding.md#destination-encoded-files) — that
  table is canonical; do not restate it here.

## Server scope

A real control plane, justified by identity consolidation and the surface it owns (not a
minimal broker). It
owns identity, projects, access, secrets, build orchestration, gating desired-state commits (by
GitHub push permission), and the kube-token + secret brokering. It **triggers/feeds** the reconcilers (via git + OCI) and
**never reconciles** in-cluster state itself — pull-only, no inbound cluster creds.

## Identity & access

- Internal **`users`** are the anchor; **`identities`** rows link external accounts
  (`provider`, stable `provider_id`, `metadata` jsonb, `kind` login|service). GitHub is
  the only adapter in v2; Google/Sentry/custom slot in later with zero schema change.
- Auth providers are pluggable behind an `IdentityProvider` interface, never hardcoded to
  GitHub; a generic OIDC adapter later covers most. Authorization is **GitHub-derived with
  zero platform RBAC** — no permission table, no roles; deploy authority is git push
  permission on the infra repo. See
  [platform-server-github-app-zero-rbac](../decisions/2026-06-29-platform-server-github-app-zero-rbac.md).
- Platform issues its own session token; all downstream (CLI, kube-broker, tf-provider)
  consume platform identity.
- Same verified email across trusted providers auto-links to one user; per-provider
  `trust`
  + `email_verified` gate it (no auto-merge on unverified/untrusted).

## Phase boundaries

- **v2** — single home cluster; platform in-cluster; GitHub-only IdP; secrets via
  platform-pull init-container; `tf/` manual; no DNS.
- **v2.1** — DNS (Cloudflare via `tf/`), PR/branch preview instances (infra CUE + namespacing),
  platform-run tofu. Gating stays GitHub push permissions — no separate approvals/plan-gate UI.
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

- Monorepo migration sequencing (touches test harness, Dockerfile, scaffold).
- v2.1 / phase-2 scope detail (DNS, branch deploys, multi-cluster agent protocol).
