<!-- not spec/decision because: append-only session journal — story, not state; never
read on resume, archaeology only -->

# LOG — session journal (append-only)

Newest entry first. Current truth lives in [STATE.md](STATE.md); walk statuses in
`*.ledger.md`. This file is never read during a resume.

## 2026-07-18 — flux Receiver cluster-wide fan-out + release v0.9.15

- Interrupt session (bugfix + release), not the srv overhaul. Started an ace-connect bridge
  (`prod9.platform.claude`, control mode); prod9/infra shared its multi-tenancy plan
  (`/tmp/flux-multitenant-plan-prod9.infra.claude.md`).
- **Platform side of the plan landed:** baseline Flux `Receiver` → cluster-wide unconditional
  fan-out (`name: "*"`, ns dropped) so one baseline-owner Receiver pokes every tenant's
  OCIRepository off the shared org webhook. The owner-vs-tenant emission role is a **convention
  in docs** (chakrit: "B) is just an instruction and/or skill/docs edit … no code"), NOT
  scaffold-mode code — honors the flat-baseline law. Commit `65d8564`; spec'd in
  `scaffolding.md` + `config-allocation.md`. Smoke UNCHANGED, guard green.
- **Two mistakes, both surfaced + corrected:** (1) my first commit swept pre-existing staged
  changes (ADR deletion, LOG rename) into the flux commit; chakrit kept it as-is. (2) I quoted
  the next release as `v0.9.10` off **stale local tags** — `platform release` fetches remote
  tags first, real latest was v0.9.14 → cut **`v0.9.15`**. Rejected a `--force` release; chakrit
  clarified there was no operator WIP — the dirty tree was a prior session's uncommitted work.
- **Committed that prior session's uncommitted tree** in 5 coherent slices (`8bf4f1f`..`39ddbe6`:
  CLAUDE.md laws, flux-webhook de-confusion T1–T4, trail split to `.ace/`, srv-rbac scratch,
  webui preview). Then cut + pushed **v0.9.15** clean (no --force). Peer notified.

## 2026-07-18 — srv 1-by-1 walk COMPLETE

- Resumed the srv 1-by-1 in a fresh Opus session from the recorded cursor (item 4 details).
  Closed the whole walk: all 12 items SETTLED or self-resolved (statuses/verbatim in the
  `.ace/` ledger).
- Key rulings this session: **item 4** rolling-install (`GET /api/install` state list; DB
  probe `SELECT 1;`; server boots with no hard deps — the "DB stays fatal" line was an agent
  derivation, retracted); **item 5** install gate = a separate **installer fx fragment**,
  first-user must GitHub-auth as org owner, install record singleton, installer→product =
  restart; **item 6** embed stands (single-origin webui), install page canonical; **released
  ruling 1 → `/api`** (webui owns `GET /*`; bare `/health`,`/auth/*`,`/hooks/*`); **item 8**
  Flux→srv = read-only (no Flux→srv webhook); **items 9+12 merged** = event-sourced build
  reconciler (`BuildEvent` stream; state = f(history, `platform.toml` timeout)); **item 11**
  zero-RBAC **confirmed** (cluster-view via GitHub rights + cluster-side provenance discovery
  + a fat caching session; no informer; app→infra handoff stays manual).
- Recurring failure this session: conflated separate concerns repeatedly (flux-webhook
  direction, GitHub→Flux vs Flux→srv; grounding design on disposable AFK srv code; treating
  scratch as frozen; over-executing mid-walk). Corrected each; the flux-webhook conflation
  is now a committed, self-contained de-confusion task list for a next session to execute.
- Two committed docs added: `2026-07-18-srv-rbac-observability.md`,
  `2026-07-18-flux-webhook-deconfusion-tasks.md`. Nothing committed/pushed (autonomy
  suspended); staged changes are chakrit's to land.

## 2026-07-17 — walk paused, save for fresh-session resume

- Item 4 details were presented post-cutover; chakrit stopped before ruling
  ("no. ace-save first") — a stop signal, not a ruling; item 4 details stay open.
- Resume plan: fresh session, Opus, `/ace` → STATE + ledger, cursor at item 4 details.
- Nothing committed anywhere this session (platform, school, dotfiles all carry
  uncommitted edits; editnotes mark the two outside clones).

## 2026-07-17 — trail v2 cutover

- Incident: the resumed walk argued against the recorded /install items (3–5), then
  confabulated context loss when challenged; separately escalated an out-of-remit
  flux-source question. Dissection + fix:
  [2026-07-17-trail-fix-plan.md](2026-07-17-trail-fix-plan.md).
- Applied: [srv-1by1.ledger.md](srv-1by1.ledger.md) seeded (item 3 SETTLED, item 4
  settled in shape, ruling 3's /setup→/install amendment recorded); STATE/LOG cutover;
  school skills (ace-save, ace, ace-connect) + user 1-by-1 + global/repo CLAUDE.md
  edited, editnote.md left in the school + dotfiles clones; the /tmp onboarding-shape
  peer note stamped RETRACTED.
- Walk resumes at item 4 details + item 5.

## 2026-07-12 → 2026-07-17 — archived resume (verbatim; was "Resume — 2026-07-12
   (CUE-module-path reshape COMPLETE)")

## ⏩ Session 2026-07-17 (docs pass) — 5 commits, unpushed, tree clean

`6a11d7e` golden re-record · `82fe12a` CLAUDE.md cut · `898dd86` spec reconcile ·
`775a0fe` srv paths + RBAC reopen · `7014916` RBAC retraction. **5 ahead of `gh/main`;
push awaits say-so.** `go test` green, smoke UNCHANGED.

- **CLAUDE.md 427 → 174 lines.** It was a second copy of `docs/spec/` and the two had
  drifted (skill list vs `ace.toml`; `versions` in neither). `docs/README.md` already names
  spec/ the source of truth, so the duplicate is gone: CLAUDE.md now carries **only Laws,
  conventions, how-we-work**. Moved out first, never dropped — Node/pnpm rationale →
  `spec/frameworks.md`; smoke drift-detector + suites + timeout → **new**
  `spec/testing.md`; `cmd/` subpackage convention + root flags → `spec/architecture.md`.
  **Rule going forward: nothing leaves CLAUDE.md until it exists in spec.**
- **The 1-by-1 record is reset** — see below; nothing settled, chakrit's rulings are givens.
- **Specs reconciled against their own rulings.** `platform.md` taught Project roles in its
  thesis/walkthrough/identity section while its own subsystem section said zero RBAC →
  fixed to GitHub-derived throughout. Baseline destination-encoding was restated **3×**
  (canonical: `scaffolding.md`); phase boundaries byte-identical in 2 files. The fragment
  reorg had moved **every** file `platform-server.md` cited without updating it — all
  repointed.
- **Deleted 3 machine-local memory files** duplicating CLAUDE.md (blackbox-first,
  narrow-cut, cmd-subpackage) per the "never duplicate what a repo records" rule. The
  cmd-subpackage one was rescued into `spec/architecture.md` first — it had no other home.
- **Two audit-agent passes:** staleness (clean — all 7 build-path specs match code) and
  routing/duplication (no misfiled docs; the 3 duplications above).

**Watch:** two findings this session came from **my own audit script false-positiving** on
prose it grepped literally (the containerd toll line, `platform-server.md`'s status). And
the claimed secrets↔RBAC ADR conflict was **word-matching, retracted in `7014916`** — don't
rebuild it.

The CUE module path is now a **scaffold-time input, not a persisted `platform.toml` key**.
All slices landed, hermetic + smoke green, **pushed to `gh` (`05311f3`)**. Tree clean.

**State as of 2026-07-16 (later):** Flux-webhook batch **pushed** (`52e9c10`). Package
reorg landed on top (`6cfe23b..8c6f10c`, unpushed): `gitctx`+`gitcmd` → one `git` package
(`git.Context`, `git.IsRoot`; dead `Describe`/`WalkSubdirs` dropped), `fileutil` folded
into `framework` (`detectFile`), bare `internal.Multiplexer` folded into `engine`, and
`cmd/` split into per-subcommand subpackages each exporting `Cmd` (`cmd/init` is package
`initcmd` — Go reserves `init`). Smoke UNCHANGED. Remaining open: `./infra` `[vars]`
migration (operator's own repo/skill).

**Reorg round 2 (same day, operator corrections):** cmd/ split walked back — subpackages
only for multi-file subcommands (today: `cmd/init` alone); singles back in `package cmd`
(`c1e1a9b`). `git/` split into `git.go` (package funcs) + `context.go` (`git.Context`)
(`42ac610`). `dsl/` nested as `gitops/dsl/` — sole production consumer (`bdc0df9`).
`project/` → **`conf/`**: `conf.Model` (was `Project`), `conf.Load` (was `Configure`),
`ModelDefaults` (`098fd4c`); specs + CLAUDE.md swept in-slice. Smoke UNCHANGED. All
unpushed.

**Round 3 (skel + docs):** `framework/skel/` now owns the one embed of shipped file
assets — the init launcher (`skel.Launcher`, renamed `platform.launcher`; was
`cmd/init/platform.template`) + the Infra baseline components (`skel.Read`; was
`framework/infrabase/`); ownership stays with the readers (`bc4ad1c`). Core packages got
package docs; stale framework/gitops docs rewritten (`d050950`). Docs audit vs new
structure fixed real pre-existing spec drift: frameworks.md five-method contract (now
six, `RequiredScaffoldInputs`), driver-side resolution + `NeedsGitRepo` remnants,
scaffolding.md's "init runs git init" + walk-up claim (git.IsRoot checks the dir
itself), releases.md/engine.md stale file pointers (`18f7a41`). Smoke UNCHANGED.

**13 commits ahead of `gh/main` — push awaits say-so.** Remaining open: `./infra`
`[vars]` migration (operator's own repo/skill).

## Landed this session (pushed)

- `0f05cef` **docs: zero-assumptions Law + ALWAYS_YES fix** (`CLAUDE.md`). Verify-before-assert,
  trace the whole producer→consumer path, keep specs current in-slice, amend the misleading
  artifact same-turn. `ALWAYS_YES=1` only auto-answers yes/no gates; value inputs come from
  positional args.
- `bde08a6` **cuemod: extract cue.mod reader** into `platform.prodigy9.co/cuemod`
  (`Present`/`Path`) — breaks the `framework → gitops` cycle for render.
- `617a317` **render: load apps by `<module>/apps`** read from `cue.mod` (byte-identical;
  fail-fast on missing cue.mod). Module path comes from cue.mod (operator truth), never
  platform.toml.
- `9a0b248` **init: remove NeedsGitRepo** — git is universal, so it was `IsInfra` in disguise.
  Platform never runs `git init`; `validateDir` checks `IsGitRoot` uniformly at init start; the
  operator inits first. `testbed.sh` git-inits testbeds in place (non-polluting; gitignored).
- `1238a50` **framework: rename baseline/ → infrabase/**.
- `69a3485` **docs: smoke/go-test are ungated completion work** — running `./test.sh`/`go test`
  never gates on the operator; the "heavy run" go-ahead rule is for model pulls, not the test
  suite.
- `0ad28ea` **framework: CUE module path is a scaffold input** — the big one. `Framework` gains
  `RequiredScaffoldInputs(wd)` (declares inputs to prompt, by name); the driver stays
  framework-agnostic (no CUE_MOD_PREFIX→ModulePath routing in the driver). Infra declares
  `CUE_MOD_PREFIX` greenfield only, validates it is a legal CUE module path; the 6 onboarding
  frameworks embed `noScaffoldInputs`. Dropped `Project.ImportPrefix` + the `Generate` write (was
  write-only). ADR rewritten+renamed to
  [`…is-a-scaffold-input`](../decisions/2026-07-12-cue-module-path-is-a-scaffold-input.md); specs +
  CLAUDE.md updated. Smoke re-recorded (infra-init `platform.toml` drops `import_prefix`; its test
  now wipes the target so init generates fresh, not merges a stale file).
- **`framework: Scaffold owns resolution` (Design B)** — `Scaffold(ctx, wd, repository,
  daggerVersion, inputs)` now returns the framework's **resolved** files; the `ScaffoldData`
  interface method is gone (folded into Infra's `Scaffold` as an unexported helper), `Resolve` +
  `Data` are Infra-internal, and the driver just writes finished bytes. Behavior-preserving
  refactor (smoke UNCHANGED). Chosen over the lighter Design A per the "abstraction outranks
  churn" law.

Smoke: **verified green** (`./test.sh` UNCHANGED after re-record; ~20–30s runs — not a heavy job).

## Open / follow-on (not this work)

- **Items 5/6 — Flux webhook delivery + guard — DONE (unpushed, awaiting say-so).**
  Infra baseline now ships push-first delivery: a github-type Flux `Receiver` on
  `registry_package` + its empty-placeholder HMAC `Secret` + an `HTTPRoute` exposing
  notification-controller's `webhook-receiver` at `FLUX_HOSTNAME`; OCIRepository poll dropped
  `1m → 10m` (webhook primary, poll = fallback). `FLUX_HOSTNAME`/`PLATFORM_HOSTNAME` joined
  `DefaultVars` as the **first `@tag` holes in CUE apps** (`platform.cue` `#host` was a literal);
  gateway coords stay literal. Guarded by `TestEmbeddedFluxReceiver` + at-site comment;
  [ADR 2026-07-13](../decisions/2026-07-13-flux-webhook-delivery.md); specs + smoke golden
  current. Verified end-to-end via `infra-init` render.
  - **Commits (all unpushed):** `977636e` feature → `9b1ce7f` breadcrumb → `c39b7df` **defs-style
    rework** (per operator: reuse `defs.#Secret`/`defs.#HTTPRoute` + `parts.#Metadata`, local
    `#OCIRepository`/`#Kustomization`/`#Receiver` defs-style scoped to the file, names owned by the
    resource via `.#name`; Secret now base64 `data` not `stringData`; smoke UNCHANGED) →
    `5763e7e` ace.toml skill swap → `18ab156` fmt. Plus earlier `f77134e`.
  - **Deferred to `srv`:** platform auto-configuring the GitHub-side webhook (needs the GitHub
    App); operator wires it by hand until then. **`./infra` not touched** — operator's re-scaffold
    picks up the new baseline.
- ~~`./infra` repo `[vars]` migration~~ **DEAD (2026-07-16)** — `./infra` is abandoned
  entirely; a completely new cluster + fresh GitOps repo replaces it. Do not resurrect.
  Bring-up runbook: [`docs/guides/cluster-bringup.md`](../guides/cluster-bringup.md);
  this session drives the infra agents via ace-connect.
- **prod9-main bring-up (2026-07-16/17):** cluster lke632028 live — gateway on reserved
  IP `139.162.23.194`, **HTTPS live end-to-end** (both hosts, real certs), flux applied
  but **suspended**. Releases v0.9.1→**v0.9.5** (v0.9.x = v2 line, ADR'd; v0.10.0
  mis-cut withdrawn). Baseline now: launcher stamps its own release (`+dirty` shapes
  handled), distributed-hosts (gateway + cluster-issuer components, ListenerSets),
  cert-manager `--enable-gateway-api{,-listenerset=true}` (live-verified; the
  feature-gates flag was wrong), allow-acme-solver netpol in flux-sync (flux stock
  netpols blackhole HTTP-01 solvers), **provider-neutral** (ADR 2026-07-16 + scope:
  cert-manager/gateway/ListenerSet stack = cross-cloud convention, stays; only
  cloud-named annotations are provider wiring, living as infra-repo edits).
  `set-unless-empty` REJECTED; **DSL changes hard-gated on chakrit — per-repo Law in
  CLAUDE.md + DSL spec header, binds autonomous sessions**. New infra repo:
  `~/Documents/prod9/infra/infra-v2` (local only, live cluster state committed there).
- **GO-LIVE COMPLETE (2026-07-17):** `prod9/infra` reused (old history archived as
  `legacy-stage9`, infra-v2 force-pushed as main), package cleaned + UI-connected to the
  repo, flux unsuspended, e2e webhook verify closed. Fixes shipped en route: v0.9.6
  single-layer artifact (14 per-file layers made Flux apply one file and PRUNE the
  cluster — recovered; one-layer contract in the plain-image ADR + CLAUDE.md), v0.9.7
  https `org.opencontainers.image.source` label (scheme-less value never linked the
  package → webhooks deaf). Measured: GitHub `registry_package` delivery is
  minutes-scale (~6m steady, 10–24m surges); corrected in ADR/runbook/skel. **Delivery
  path fully live: commit → publish → webhook → reconcile.** Remaining migration phases:
  DB restore + app onboarding (backups in old infra repo's `bak/`), defs pack-level
  `#UseCertManager` default (routed to `prod9.infra-defs.claude`; platform.cue carries
  the interim mixin), stage9/lke15414 teardown LAST, then post-roadmap deploys (lem,
  geekshop, axa-bluestar).
- **🎯 NEXT (fresh session): RESUME the srv 1-by-1 — IN PROGRESS, nothing settled.**
  The walk ran 2026-07-17 but chakrit closed out none of it. His correction (2026-07-17):
  *"the 1-by-1 is IN PROGRESS and only like a few items have been settled"* — the few
  being his own pre-walk rulings, not the walk's output. This bullet previously read
  "EXECUTE the settled srv 1-by-1 / every item is decided" and misled the next session
  into proposing spec drafting; that claim was false and is retracted.
  Record: [`2026-07-17-srv-1by1.md`](2026-07-17-srv-1by1.md) — **canonical home for his
  standing rulings** (do not re-state them here; the duplicate is how they drifted) and
  for the walk's UNSETTLED proposals.
  Next session = re-walk the open items with the `1-by-1` protocol, chakrit driving each
  close-out. Two entry points, both his to answer:
  1. **The `/setup` vs `/install` collision** — the walk renames the concern to `install`
     (`GET /install`) while his ruling 3 names `GET /setup` in the exact set. The prior
     session recorded the override as decided without ever surfacing it.
  2. **Item 11 — RBAC, reopened by chakrit 2026-07-17** ("the RBAC thing we need to
     revisit"). The agent found **no defect** in zero-RBAC; a claimed secrets/RBAC ADR
     conflict was word-matching and is retracted. So *ask him what he wants revisited* —
     do not re-derive a justification. If zero-RBAC reverses, `898dd86` is the revert point.
  Do **not** draft specs/ADRs from the record and do **not** hand it to a subagent —
  drafting is downstream of a closed walk. (Execution mode once it closes, per chakrit:
  write the changes into specs/ADRs/guides first; an unattended subagent implements from
  those later.)
  Scope of the re-analysis: `srv/` (routes in `srv.go` Router + fragment controllers,
  fragment action granularity, `Serve` boot sequence), plus the ops table in
  `docs/spec/platform-server.md` and the webui fetch paths (`/api/me`, `/api/builds`,
  `/api/auth/*`) which all move when the prefix drops.
- **Pending school change (for `ace-school`, from the srv run):** `prod9-fx` skill —
  document the programmatic migrator boot pattern (`migrator.New(db, FromFS(fs))` →
  `Plan(ctx, IntentMigrate)` → per-plan `Apply`; dirty = refuse non-interactively; used
  by `srv/migrations.go`) and that `render.JSON` is 200-only (a non-200 JSON body needs
  a local writer — see `srv/webhooks.go` `renderAccepted`). NOTE 2026-07-17: hold until
  the 1-by-1 above settles — boot-time auto-migrate itself was ruled out, so the pattern
  to document may change shape (manual/systems-dashboard trigger instead).
- **Pending school change (for `ace-school`, candidate — settle via the 1-by-1 first):**
  chakrit's cross-project architecture prefs from the srv review (bare backend routes,
  RESTful resource naming, big convergent actions, interval-worker over boot-time
  special-cases, manual/dashboard-triggered migrations) → route into `general-coding`
  (API/architecture section) and/or `prod9-fx` once the walk confirms final shape.
- **Session 2026-07-17 (this one) — landed:** school PR
  [#67](https://github.com/prod9/school/pull/67) (prod9-fx fragment self-containment;
  open). Launcher reworked twice with the infra agent over ace-connect: go-install
  (b781ca0) then **v2 version-in-filename, awk-free** (b2a8d46) — `bin/platform-<ver>`,
  self-gitignoring, existence check = version check. `platform versions` table +
  root `--version` (raw stamp only — derived-release reporting REJECTED). Go →
  1.26.5. **v0.9.13 released + pushed**; infra agent notified to bump their pin.
  Unpushed after the release: spec ops table (47482dd), containerd-vs-dagger research
  scratch (59d7015, `docs/scratch/2026-07-17-containerd-vs-dagger-engine.md`, verdict
  feasible-but-costly), **srv reorg into self-contained fx fragments** (srv/auth,
  srv/github, srv/builds, srv/flux + pgerr/migrate/srvtest; api.go dissolved;
  CreateSession hashes internally), **webui slice** (SvelteKit + p9-brand, self-hosted
  woff2 fonts 3MB, committed build/ embedded; p9-brand wired into ace.toml). All tests
  green, smoke UNCHANGED at end. ⚠️ The srv reorg + webui slices are the very code the
  1-by-1 will rework — don't polish them before the walk. Live-server/visual verify
  still owed (needs operator). ace-connect engine `prod9.platform.claude` live,
  control mode, pid from pre-/clear session.
- **Pending school change (for `ace-school`):** `p9-infra` skill says defs current =
  v0.3 — stale; v0.4.0 is live (baseline pins it): `#NetworkPolicy` access-grant
  pattern, `#ListenerSet` + `#AllowListenerSets`, `packs.#WebApp` emits its own
  ListenerSet, `#HTTPRoute` `#listenerset_name`. Skill needs a v0.4 section/routing.
- Session mechanics: ace-connect engine `prod9.platform.claude` (control mode) live with
  `.inbox.log` in repo root; nvim syntax file at `~/.config/nvim/syntax/platform-dsl.vim`
  (skel `.platform` files carry `# vim: ft=platform-dsl` modelines).
- **Post-go-live stretch (2026-07-17, later):** x9 CUT OVER (live on prod9-main, per old
  repo's migrate guide). Releases v0.9.10–v0.9.12: NGF pin → v2.6.7; gateway-api channel
  → **standard-only, exp variant deleted** (operator ruling; ListenerSet is standard/v1 —
  verified in the release artifact); `defaults/webapp.cue` wrappers (`#gateway` coords
  once, `defaults.#WebApp`/`#Listeners`); semver-aware release sort (string sort broke at
  v0.9.10); publish `org.opencontainers.image.source` https fix (package↔repo linkage);
  single-layer artifact fix. **TCPRoute root cause found:** stage9 pmrelay died from a
  missing `--gateway-api-experimental-features` NGF flag, not an NGF gap — two-edit
  revival recipe in the vendor NGF doc. README rewritten; editor/nvim vendored. Laws
  added: DSL changes hard-gated on chakrit (CLAUDE.md + spec header). Engine restarted
  with updated ace-connect (peers-no-authority/NACK). App onboarding continues
  agent-driven per /tmp/app-onboarding-shape-prod9.platform.md.
- **Next work threads (queued 2026-07-17, in order):**
  1. ~~The CI/CD server component (`srv`)~~ **DONE (2026-07-18 AFK run, c6619da..b8eb880,
     unpushed):** 8 slices — serve skeleton + webui embed seam, DB + users/identities,
     GitHub App manifest bootstrap, webhook ingest → queued builds, repo-prep
     bare-mirror cache, build runner (engine's second driver; `BuildAndPublish` now
     returns results), user OAuth + sessions + `/api/me`,`/api/builds`, installation
     tokens + flux-webhook auto-config (closes the flux ADR's deferred half) + a
     two-phase batch audit remediation (git.Run boundary, push-permission gate,
     orphan requeue, boundary test). All hermetic+DB tests green, smoke UNCHANGED
     throughout. Live-server verify (real dagger path, real GitHub App) still owed —
     needs an operator. Blockers/notes in `.afk.log` (p9-brand skill missing; reads
     unaddressed by zero-RBAC record).
  2. The CI/CD server UI (SvelteKit plain JS into `webui/build`; **use p9-brand skill**
     — not installed on this machine yet).
  3. Ingest fi-build's `build.py` scripts → make a platform framework out of them.
- **After the entire CI/CD roadmap is done:** deploy `lem`; deploy `geekshop`; deploy
  `axa-bluestar` (three separate tasks, all gated on roadmap/migration completion).
- ~~07-11 audit leftovers~~ **CLOSED (2026-07-16)** — sweep of the lexicon ADR's rulings found
  all code renames landed; sole leftover was one stale word in `docs/spec/README.md`
  (`latest` → `rolling`), fixed in `da0ea2d`.
- ~~School change: abstraction-outranks-churn~~ **DONE (2026-07-16)** — folded into
  `general-coding` as "Churn Never Outranks the Boundary";
  [PR #61](https://github.com/prod9/school/pull/61) open, school cache on branch
  `ace/churn-outranks-boundary`.

## Watch (design laws that bit this session)

- **No app-vs-infra branch in `init`** — Infra's differences are framework-declared data
  (`RequiredScaffoldInputs`, like the removed `NeedsGitRepo`), consumed generically. Never `IsInfra`.
- **Narrow cut**: `RequiredScaffoldInputs` has one real consumer (greenfield Infra). Generalize to
  a set only when Go/pnpm actually scaffold fresh module files.
- **Stale testbed files**: infra-init has no committed `platform.toml`, so leftover gitignored
  files make init *merge* not *generate* — its smoke test wipes the target first.
