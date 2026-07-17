# PRODIGY9 Coding School

This project's AI coding environment is managed by [ACE](https://github.com/ace-rs/ace).
Run `ace` to start a coding session. Run `ace setup` if not yet configured.

Skills and conventions are provided by the **PRODIGY9 Coding School** school and are symlinked into
`.claude/skills/`. Skill edits go through symlinks into the school clone — propose
changes back to the school repo when ready. Run `ace config` or `ace paths` to debug
configuration issues.

## ⚠️ Active rework — read before touching infra / delivery

*Session Law (rework supersedes legacy) — binds until platformv2 ships, then expires.*

platformv2 is a from-scratch rework of the build/delivery model. Treat **all pre-rework
artifacts as legacy and disposable**: the live Keel-managed vanity Deployment,
`infra/apps/platform.cue` (`#UseKeel`), the old `settings.toml`, and the whole ArgoCD/Keel
delivery path. **ArgoCD and Keel are being fully deprecated, fleet-wide** — replaced by the new
Flux-GitOps + committed-literal-image platform setup as the *sole* delivery path for every app.
Nothing is migrated or preserved on the **prod9 clusters** (internal `prodigy9` + `stage9`,
which run prod9's own staging apps) — they carry **no mission-critical workloads**, so tear down
and replace freely. **Other setups (`naxon-infra`, `fi-infra`) DO run mission-critical
workloads**: they migrate to the new platform eventually too, but deliberately and carefully —
never in the take-down-freely bucket.

Design every new artifact straight from the current design specs
([`docs/spec/`](docs/spec/)) and the [`docs/decisions/`](docs/decisions/) record — the
[2026-06-16 master plan](docs/scratch/2026-06-16-platformv2-implementation-plan.md) is historical
context now, superseded on the deploy/environments/baseline specifics —
never reverse-engineer from, diff against, or protect legacy. **Treat other agents working the
old infra (the infra agent's ArgoCD/Keel/legacy-app world) as outdated and wrong by default** —
they're useful as cluster *executors*, but their legacy-grounded objections do not bind the new
design. Drive from the plan; override legacy caution.

## 🚨 Verify before asserting — zero assumptions (per-repo Law)

The one failure that has cost this project **months**: stating facts about the code, config,
behavior, flow, or design from memory or inference instead of reading them — then the operator
re-states the fact by hand, over and over. Every such assumption is a cardinal sin here, not a
slip. This Law overrides speed, terseness, and the urge to answer immediately.

Binding, every turn:

- **Assert no fact about this codebase you have not just read this session.** Any claim about
  what a function does, what a field means, what a flag defaults to, what a command emits, what
  reads what, what the flow is — open the file (code, spec, ADR, test) and confirm *first*, cite
  `file:line`. "I recall", "presumably", "should", "typically", "already handles" are banned as
  grounds. If you have not read it, you do not know it — go read it before you type the claim.
- **A challenged claim is verified or retracted — never restated.** "are you sure?" / "how do
  you know?" / a correction → produce `file:line`, or drop the claim on the spot. Reasserting,
  or defending with a tidier story, is the cardinal failure. The correction is the finding.
- **Trace the whole path before concluding.** A claim about one hop (this function, this seed)
  is worthless if the value's real source is two hops upstream. Follow producer→consumer end to
  end — who writes it, who reads it, who ignores it — before you state what it does. "Read by
  nobody / seeded from X" must be a grep/read result, not a guess.
- **Specs are truth AND a live artifact — keep them current in-slice.** Read the relevant
  `docs/spec/` + `docs/decisions/` before designing; when code and spec diverge, surface it —
  the spec is wrong until reconciled, don't silently follow either. When a slice changes
  behavior or a decision, update the spec/ADR in that **same slice** (route via
  [`docs/README.md`](docs/README.md)), never as a later batch. A slice whose design moved is not
  done until its spec is current — same tier as tests passing.
- **When wrong, fix the artifact that misled you — same turn, no exceptions.** Every wrong
  assumption traces to a source: a `CLAUDE.md`/spec/comment line that stated it imprecisely, or a
  silence that let it stand. Amend that source the moment the error surfaces so a fresh session
  can't repeat it — a corrected line here is worth more than any single fix. If the trip was pure
  inattention with no misleading artifact, sharpen this section instead. (E.g. the `ALWAYS_YES`
  clarification below was added exactly this way.)

## 🚨 DSL changes are hard-gated (per-repo Law)

Any change to the manifest-patch DSL (`gitops/dsl/` — verbs, grammar, semantics, its
spec) requires chakrit's explicit approval, in every session, autonomous ones included —
no standing grant ever covers it, no exceptions. The DSL is deliberately small and
branch-free; we should never need to change it. A proposed change needs a really good
reason, presented and approved before any edit.

## Conventions

Commit messages **(per-repo Law)**: `area: Capitalized description`. Prefix is a code component/topic (`deps:`,
`docs:`, `tooling:`, `cmd:`, `kubectl:`), not a skill or tool name. Capitalize the description;
put clarifiers in parens at the end; never a `(scope)` in the prefix. Keep the `Co-Authored-By:
Claude …` trailer. Not `type(scope):`.

`ALWAYS_YES=1` **only auto-answers yes/no confirmation gates** (fx `prompts.Confirm`/`YesNo`) —
it is NOT headless mode and does NOT feed value prompts. Init's value inputs (maintainer, email,
repository, …) come from **positional args** to `platform init` (fx `prompts.Str` consumes
`s.args` in order); a value prompt with no arg still blocks on stdin regardless of `ALWAYS_YES`.
So non-interactive init = pass the values as positional args **and** set `ALWAYS_YES=1` for the
final confirm (see `tests.cue` init invocations). `--force` is unrelated: it means "replace
existing files" (write disposition), not prompt suppression.

## Design approach — how this project cuts

The recurring failure when working here is **conflating concerns the design keeps separate**,
and **adding code for the remainder** the design discards. Counter both:

- **One high-ROI concern per unit; discard the rest by convention, not code.** A verb, a
  package, a command does exactly one thing. The parts it does *not* handle are covered by a
  stated convention (a rule you follow), not by a guard, a flag, a helper, or an extra
  command. Absence of scaffolding is deliberate, not an omission to fix. (E.g. no
  release-but-unpublished guard — it's fine by convention.)
- **Separate the domain from the mechanism.** When two things co-occur, that is often a
  coupling of the *mechanism* that happens to run them together, not evidence they are the
  same concern. Ask "domain or mechanism?" before merging. Default to the **narrower cut**.
- **Don't be exhaustive.** Completeness, symmetry, and "while I'm here" bundling are the smell.
  Build the narrow thing; leave the rest to convention. When unsure whether a part is in
  scope, it is probably out — ask, don't add.

Delivery verbs are the canonical instance: `release` (cut a tag) and `publish` (build + push an
image) are **orthogonal — neither implies the other**. There is **no `deploy` verb** and no
platform-managed `environments`: in the pull model "deploy" is the operator committing the infra
repo, then `publish` (with a platform server + Flux) or `render` + `kubectl apply` (no
server); multi-env lives in the infra CUE (a template instantiated per env) + k8s namespacing,
gated by GitHub push permissions. One publish engine (`engine` package), two drivers: the local
CLI now, a tag-watch server later. See
[delivery-verbs-are-orthogonal](docs/decisions/2026-07-05-delivery-verbs-are-orthogonal.md).

## Build & delivery facts

`publish` builds `publish_arch` (default `amd64`) so an arm laptop never ships an unrunnable
image; local builds (`build`/`preview`/`export`/`ls`) use `local_arch` (default `auto` = host
arch). `publish` authenticates to ghcr via the **local docker credentials** (osxkeychain) — no
`REGISTRY_USERNAME`/`PASSWORD` needed for a local push. What a cluster runs is the app-image ref
**committed in the infra repo's git** (the operator hand-edits it — platform never rewrites their
CUE); pin that ref by `tag@sha256` digest to dodge stale node cache (`IfNotPresent` won't re-pull
a moved tag). The record is the git commit, not the tag's immutability.

The infra delivery image must carry the rendered tree in **one OCI layer**: Flux's
source-controller extracts a single layer per artifact — a multi-layer image delivers one
file and `prune` then wipes the rest of the cluster (bit prod9-main once). `Infra.Build`
enforces this (one `WithDirectory`); never revert to per-file `WithNewFile` on the
container.

platform self-delivers from the **`prod9/infra` GitOps repo** (working copy
`~/Documents/prod9/infra/infra-v2`; module `prodigy9.co`) onto the prod9-main cluster —
`./infra` in this repo and the old stage9 deployment are dead legacy.

**Node/pnpm provisioning is deliberate — never "simplify" it to distro packages.** pnpm
frameworks take Node from the official nodejs.org build via tj/n, and pnpm via Node's own
corepack — **never** `apk add nodejs`/`corepack`. Node, corepack, pnpm, and the distro are
four uncoordinated maintainer groups; sourcing Node from the distro adds a party whose
repackaging borks the seams downstream (linux-wifi-driver style). Stay closest to the
least-magic, most-reliable upstream — good taste in tech. pnpm over npm because npm is slow;
corepack because it's Node-sanctioned and narrow-jobbed, not a version-juggler like nvm. A
cache or build failure in this step is **never** a reason to switch to apk — shed the cache
with `platform clean` (first-line diagnostics for any "worked on a fresh checkout but not
here" failure) and fix the real cause.

## lowfat (token saver)

Command output is compacted by [lowfat](https://github.com/zdk/lowfat) via a user-scope
hook — no prefix needed; output passes through unchanged when no filter matches. Project
config lives in [`.lowfat`](.lowfat); re-sync pantry filters with the `/lowfat-pantry` skill.

## Durable artifacts

`docs/` — file by the routing gate in `docs/README.md`: a ruling → `decisions/`;
third-party lookup → `vendor/`; a how-to → `guides/`; our own design/surface → `spec/`;
unsettled exploration → `scratch/` (residual, opened with a "not spec/decision because ___"
line). Nothing defaults to `scratch/`. See [`docs/README.md`](docs/README.md) and per-dir
READMEs (each indexes its files); [`docs/spec/architecture.md`](docs/spec/architecture.md)
is the entrypoint to the build-pipeline design.

## Project Overview

`platform` is PRODIGY9's self-contained build/CI tool — a Go CLI (module
`platform.prodigy9.co`, Go 1.26.5) that auto-detects project type, builds containers via
Dagger, manages releases via git tags, and scaffolds new repos with a `platform.toml` +
build script.

Goal: zero per-project build config; new repos onboard quickly; no tech-stack lock-in.

### Entry point

- `main.go` — defers to `cmd.Execute()`.
- `cmd/` — the root Cobra command (persistent `-q`/`-v` and `-f` for an alt
  `platform.toml`, subcommand wiring) plus one file per single-file subcommand. A
  subcommand with its own file cluster gets a subpackage exporting `Cmd` — today only
  `cmd/init` (package `initcmd`; Go reserves `init`). All read the config first.

### Subcommands

| Cmd | Purpose |
|-----------|------------------------------------------------------------------|
| init      | Scaffold a repo via the discovered framework's `Scaffold` — its full contribution to a fresh repo (`Infra.Scaffold` simply contributes more: the whole GitOps baseline). Alias `scaffold`. |
| build     | Build container(s) for module(s) via Dagger.                     |
| configure | Print effective parsed config.                                   |
| exec      | Run a command in, or shell into, the built container; bare+piped prints a summary (debugging). |
| export    | Build and export container as `.docker` tarball.                 |
| ls        | Tree the source files going into the container (debugging).      |
| preview   | Build and serve container locally via Dagger tunnel.             |
| render    | Render an `apps/` tree (CUE + `.platform`) to a `k8s/` manifest tree. |
| publish   | Build+publish a module's image under its strategy's tag (versioned → the release tag; `rolling` → the moving `latest` tag). |
| release   | Create new release tag (semver/timestamp/datestamp/rolling); supports `-p/-m/--major`. |
| clean     | Prune the local Dagger build cache (first-line cache diagnostics).|
| serve     | Start the platform server (`srv/`): embedded web UI at `/`, API under `/api/`. |
| vanity    | Hidden HTTP server: redirects `go get platform.prodigy9.co` to GitHub. |

### Packages

- `conf/` — the config model, owns `platform.toml`: parse, generate, merge. `Model` (maintainer,
  repository, strategy, excludes, modules, `[vars]`) and `Module` (workdir, framework —
  legacy `builder` key read as a deprecated alias — env, port, cmd, args, asset_dirs,
  build_dir, image, package). The publish target is not a stored section: a module's image
  is inferred per-module from `repository` (`ghcr.io/x`, `InferImageBase`) with `[modules.x.image]`
  the explicit override, and the tag derives from the release strategy (`rolling` → `latest`;
  versioned → the release version). The **CUE module path** (cue.mod `module:` + app-import
  prefix) is NOT a `platform.toml` key: it is the `CUE_MOD_PREFIX` scaffold input the operator
  supplies at init (greenfield) or is read from an existing `cue.mod` — a **separate** concern
  from `repository` (GitHub host); `cue.mod` is its sole home (see the
  [cue-module-path ADR](docs/decisions/2026-07-12-cue-module-path-is-a-scaffold-input.md)).
  `[vars]` (top-level) is the verbatim DSL `\(var)` table —
  a generic `map[string]any` (values keep their TOML type), pure passthrough
  (no defaults/inference), consumed project-wide by `render`.
  `Load(wd)` walks up to find file,
  applies defaults, env overrides (`PLATFORM`), and inferred values (e.g. `ghcr.io` image
  name from `github.com` repository). `Generate` writes a fresh `platform.toml`; re-init
  folds default `[vars]` in via the surgical line-by-line merge (append new keys,
  preserve operator values + comments/order — never decode/re-encode).
- `framework/` — a `Framework` is the **sole owner of a project
  type**: it recognizes itself, scaffolds itself, builds itself. See
  [spec/frameworks.md](docs/spec/frameworks.md) + [spec/scaffolding.md](docs/spec/scaffolding.md).
  - Contract: `Name/Layout/Discover/RequiredScaffoldInputs/Scaffold/Build`.
    `RequiredScaffoldInputs(wd)` declares the operator inputs the framework needs at init (by
    name, the prompt label — most onboard an existing repo and need none; embed `noScaffoldInputs`);
    `Scaffold(wd, env, inputs)` (`env` = `scaffold.Env`: repository, maintainer email,
    dagger SDK version) returns the framework's complete, **resolved**
    contribution — it owns filling its own template holes (which input maps to which hole, reading
    an existing cue.mod), so the driver just writes finished bytes and never sees a hole. Runtime
    shape is a descriptive taxonomy in prose (native/bytecode/interpreted/static/custom), not a method.
  - Layouts: `basic` (single module) | `workspace` (multi-module).
  - Known frameworks (order-sensitive for discovery, `Infra` first): `Infra`,
    `GoWorkspace`, `PNPMWorkspace`, `GoBasic`, `PNPMStatic`, `PNPMBasic`, `Dockerfile`.
    `FindFramework(name)` resolves the `[modules]` `framework` key at build time — the
    build path never re-discovers.
  - `framework/scaffold/` — the **one** files/templating mechanism (`scaffold.Spec`/
    `scaffold.File`; `.tmpl` renders via `text/template` `missingkey=error`, everything
    else passes verbatim). No standalone `scaffold/` or `baseline/` package.
  - `framework/skel/` — the shipped file assets (one `//go:embed`): the `platform`
    launcher template (`skel.Launcher`; the init driver resolves its version hole with
    `framework.PlatformVersion()` — the nearest release this binary descends from, exact
    tag or pseudo-version predecessor — and writes it into every repo) and the
    cluster-baseline components the `Infra` framework picks via `skel.Read`. Storage
    only — ownership lives with the readers.
  - The `Infra` framework installs the cluster baseline: files destination-encoded by name
    (`apps-*` → `apps/`, `defaults-*` → `defaults/`, else repo root), `DefaultVars` =
    version pins + ingress hostnames. The baseline is **provider-neutral** — no cloud
    annotations/vars ever ship; provider LB wiring is the infra repo's own edit (see the
    [provider-neutral ADR](docs/decisions/2026-07-16-baseline-is-provider-neutral.md)).
    Components own their hostnames via `ListenerSet`s; the baseline ships a host-agnostic
    `Gateway` app + ACME cluster-issuer
    (`MaintainerEmail` hole). It installs **unconditionally** — no install-time picker; registry
    creds ship as empty placeholders in committed CUE, never prompted. It seeds
    `strategy = "rolling"` and needs a fresh git repo — no `IsInfra` predicate anywhere,
    the app/infra difference is pure `Scaffold` polymorphism.
  - `base.go` — Wolfi base image (`cgr.dev/chainguard/wolfi-base`), apk cache mount,
    `CacheBuster` const for global cache invalidation.
- `engine/` — the Dagger execution layer. `New`/`NewContext` open an `Engine` (a client
  pool over the discovered runners); `Build` runs an attempt's units and `Publish`
  pushes them, both fanning out via an in-package multiplexer. `BuildAndPublish` is the reusable
  build+tag+push unit that `publish` drives now and a tag-watch server drives later
  (see the [delivery-verbs ADR](docs/decisions/2026-07-05-delivery-verbs-are-orthogonal.md)).
  Registry creds via fx env config: `REGISTRY`, `REGISTRY_USERNAME`, `REGISTRY_PASSWORD`.
- `cmd/init` — the scaffold orchestration (plan-then-apply): gather operator inputs →
  `framework.Discover(wd)` → `fw.Scaffold` → print plan → confirm → write (creating the
  git repo first when the spec asks). `ALWAYS_YES=1` drives it non-interactively;
  `--force` means **replace existing files** (write disposition), not prompt suppression.
- `releases/` — Release strategies: `semver`, `timestamp`, `datestamp`, `rolling`
  (non-versioned: cuts **no git tag** — emits the moving `latest` tag, its marker in the registry).
  `Generate` diffs commits since last tag, `Changelog` formats them, `Create` tags +
  pushes. Bump vocab: `BumpAny/Patch/Minor/Major` (flags `-p`/`-m`/`--major`).
  `collection.go` recovers history from git tags. `dateref`/`timeref` subpackages parse
  the datestamp/timestamp ref formats.
- `git/` — the repo's one git-exec boundary: `git.Run` executes git in any directory
  (srv's repo-prep drives mirrors/worktrees through it); `git.Context` runs git for a
  project, caching current branch and tracking remote via `sync.OnceValues`;
  `git.IsRoot` is the repo-root probe init's validation uses. Version tags are annotated and pushed once,
  non-forcefully — git holds only immutable version tags; the moving `latest` reference
  is a registry concern, not a force-pushed tag.
- `gitops/dsl/` — manifest patch DSL (Slices D1–D2): a hermetic, line-oriented directive
  language for adapting foreign Kubernetes manifests. `Apply(directives, Options)` runs
  directives against a two-state buffer (raw bytes after `download`/`extract`, decoded
  lazily when an edit or `emit` needs docs); `Lex` tokenizes shell-style into `Token`s,
  `resolve` does escape + `\(var)` interpolation (string-only, undefined = hard error,
  `\\(` stays literal); `ParsePath` compiles the dotted path syntax
  (`Key`/`Index` steps only — `[]` is focus-only, and matching a list element by field is
  `focus`'s job, not a path selector);
  `Get`/`Set`/`Remove`/`Append` walk it. In-buffer verbs (`focus`, `reset`, `set`,
  `set-if-absent`, `append`, `append-if-absent`, `remove`, `remove-doc`) plus I/O verbs
  (`download` via `Options.Fetch`, `extract` magic-byte gzip/zip/tar, `emit` truncate-write
  under `Options.OutDir`). Checksum guard deferred past D2. Spec:
  [`docs/spec/manifest-patch-dsl.md`](docs/spec/manifest-patch-dsl.md).
- `gitops/` — infra manifest rendering (the publish half retired with oras). `Render` walks
  `apps/` and routes by extension into one `k8s/<component>/<file>` tree: `.cue` → file-map
  export via the linked CUE evaluator (`exportCue`, in-process — no `cue` binary),
  `.platform` → `dsl.Apply`. `[vars]` feed
  both routes — CUE `@tag(name)` holes (only the names a `@tag` actually declares are injected;
  the rest are directive-only) and directive `\(var)`. The image ref is a **committed CUE
  literal**, never injected (see the
  [committed-image correction ADR](docs/decisions/2026-06-26-render-is-pure-function-of-committed-git.md)).
  Wired as `platform render`; the `Infra` framework packs this tree into the published
  image, pushed by the ordinary `publish` (oras retired — see the
  [plain-image ADR](docs/decisions/2026-07-05-infra-publishes-as-plain-image-retire-oras.md)).
- `srv/` — the platform server layer: API + webhook processor above the shared packages
  ([spec/platform-server.md](docs/spec/platform-server.md), incl. the full operations
  table); started by `platform serve`. Organized as **self-contained fx-style fragments**
  (one subpackage per concern, each carrying its own domain, controllers, and embedded
  `*.sql` migrations — copy-pasteable per the prod9-fx convention):
  - `srv/auth/` — users + identities + sessions (schema per the
    [identity ADR](docs/decisions/2026-06-14-identity-and-linked-accounts.md)): the
    GitHub user-OAuth login flow (`/api/auth/github` + callback: state cookie, code
    exchange, `GET /user`, `UpsertGitHubUser` keyed on `(github, provider_id)`, user
    token `secret.Hide`'d into identity metadata), sessions (`CreateSession` takes the
    raw token and hashes internally — SHA-256 at rest; 30d `platform_session` cookie;
    logout deletes), the `RequireUser`/`CurrentUser` gate, and `GET /api/me`
    (`SessionCtr`).
  - `srv/github/` — the GitHub App: single-row `github_app` storage (fx-`secret`
    encrypted; `LoadApp` is the test seam), `/setup/github` manifest bootstrap
    (`SetupCtr`; SERVER_URL / GITHUB_URL / GITHUB_API_URL config), installation tokens
    (`appJWT` hand-rolled RS256 → `MintInstallationToken`; not installed →
    `ErrAppNotInstalled`), and `CheckRepoPath` (the owner/repo whitelist).
  - `srv/builds/` — the build pipeline: queue actions (`Create`/`Claim` via
    `FOR UPDATE SKIP LOCKED`/`Finish`/`Fail`/`RequeueOrphans`), webhook ingest
    (`WebhookCtr`: HMAC-verified `POST /api/webhooks/github` queues a row per pushed
    `refs/tags/v*`), repo-prep (`PrepRepo`: bare mirror at
    `<CACHE_DIR>/git/<owner>/<repo>.git`, fetch under flock, never shallow; per-build
    worktree at `<CACHE_DIR>/work/<build-id>/`; `RemoveWorkTree` cleans up), the runner
    loop (`RunQueued`: claim → prep → `conf.Load` → `engine.BuildAndPublish` →
    finish/fail; 2s poll, immediate re-claim; engine half seamed as `publishBuild` for
    dagger-free tests), and `GET /api/builds` (`APICtr`, last 50 newest-first).
  - `srv/flux/` — `POST /api/repos/{owner}/{repo}/flux-webhook` (`WebhookCtr`,
    session-gated + explicit push-permission check): creates the repo's
    `registry_package` webhook → cluster Flux Receiver (duplicate → 409), closing the
    flux-webhook ADR's manual step. Composes auth + github; keeping it out of either
    breaks the auth↔github import cycle.
  - `srv/pgerr/`, `srv/migrate/`, `srv/srvtest/` — postgres error classification,
    migration-source merging (root `Serve` aggregates every fragment's `Migrations`
    embed, re-sorted by timestamp), and shared test scaffolding (`SetupDB` takes the
    migration sources each fragment's tests need — srvtest imports no fragment, so no
    cycles).
  - Root `srv` composes: `Router` (chi + fx `Configure`/`LogRequests`, `/api/health`,
    embedded web UI at `/`) stays DB-free so router tests run without postgres (DB
    tests skip when DATABASE_URL is unset); `Serve` owns the DB boot (DATABASE_URL
    fail-fast → aggregated migrations, dirty state refuses boot → orphan requeue →
    data-context middleware → runner goroutine). Logs via `fxlog`, never `buildlog`.
  Only `cmd` may import `srv` or its subpackages — the shared packages stay srv-free
  (guarded by `srv/boundary_test.go`).
- `webui/` — the web UI: SvelteKit source (plain JS, Svelte 5, adapter-static; pnpm via
  corepack) plus its **committed** `build/` output embedded via `Assets`
  (`//go:embed all:build`) — rebuild with `pnpm build` and commit the result. One page:
  `/api/me` gates between the GitHub sign-in hero and the builds table (`/api/builds`,
  logout). Styling is p9-brand (`src/p9.css`: brand tokens + hand-rolled woff2-only
  `@font-face` over `@fontsource` packages — self-hosted fonts, no CDN at runtime, only
  the weights the UI sets). Dev: `pnpm dev` proxies `/api` + `/setup` to the platform
  server on `:3000`.
- `internal/` — `buildlog` (build/CLI structured logger), `buildinfo` (program *output*
  rendering to stdout — results/summaries, NOT binary build info; that's
  `debug.ReadBuildInfo` at its readers, e.g. `framework.DaggerVersion`/`PlatformVersion`),
  `timeouts` (TOML duration).
- `testbeds/` — Sample projects per framework, exercised by smoke tests.

### Testing

**Philosophy (per-repo Law): blackbox-first; test-in-build is a hard gate.** Prefer
blackbox smoke tests over many small unit tests — tests earn ROI at the boundary, so
platform leans almost entirely on `./test.sh`, with `go test` the light hermetic
complement, not the primary strategy. Building an image from red tests is a non-use-case:
green tests are a **baked-in, non-configurable** precondition of every build — no
skip-tests opt-out will be added (opinionated flow, not CI phases). See the
[test-in-build ADR](docs/decisions/2026-07-05-test-in-build-is-a-hard-gate.md).

🚨 **Running `./test.sh` and `go test` is required completion work — it NEVER gates on the
operator.** Do not ask permission to run them and do not defer them to a go-ahead; a slice
is not complete until smoke has verified it (and its golden reviewed/re-recorded). The
global "heavy run needs a per-run go-ahead" rule is for genuinely resource-hogging batch
jobs (model pulls, bulk embeddings) — the project's own test suite is **not** that, however
long a cold-cache Dagger build takes. Just run it.

Two suites, at different layers:

- **`go test ./...`** — hermetic unit tests (no docker/network, fresh-clone runnable). Runs
  inside **every image build** (the `Go*` framework gate) and locally on demand.
- **`./test.sh`** — blackbox smoke (`chakrit/smoke`): drives the built binary through Dagger
  against the testbeds; **needs docker**. Runs on the host, manually / pre-publish — the
  drift detector detailed below.

`./test.sh` → runs `cue eval tests.cue → tests.yml` → `chakrit/smoke` runner. Tests
build the binary, then for each testbed run `init`/`build` checking
exitcode/stdout/expected-files. `./testbed.sh <dir> <args>` runs platform inside a
specific testbed.

Smoke is a **drift detector**, not an assertion engine. `tests.lock.yml` is a recorded
golden of each command's *actual* output — exitcode, stdout, and the content of any
non-reserved `checks:` entry (a file glob snapshots its matched files' bytes, not just
their existence). The golden is whatever the command last produced, not a hand-authored
"correct" value: correctness is established once, by a human reviewing the diff when a
line is recorded; thereafter the test guards only against *unreviewed change*.

So a green run (`UNCHANGED`) means "nothing drifted," not "behavior is correct"; a red
run (`CHANGED`, exit 1) means output moved off the golden — a prompt to
**review the diff and decide**, not a failed assertion. Intended drift → re-record with
`./test.sh --commit`; unintended → a regression to fix at the source. Never `--commit` a
CHANGED lock unread, and never massage code just to force output back to the old golden —
both blind the detector. A slice that moves smoke output isn't done until its golden is
re-recorded and the diff reviewed, in that same slice — same tier as `go test` passing,
never a session-end batch; the docker/runtime cost is mechanism, not a deferral.

The 1m per-test timeout in `tests.cue` is deliberately tight — it keeps builds honest.
Never raise it to make a slow build pass: fix the slowness (cache reuse, unnecessary
work, network pulls) instead, since a slowdown landed by one person taxes everyone's
local and CI cycles. Cold-cache pulls of a freshly pinned image are the one accepted
cause — verify by warming the cache and re-running, not by touching the timeout.

### Key dependencies

`dagger.io/dagger` (container builds), `fx.prodigy9.co` (config + cmd prompts +
ctrlc), `BurntSushi/toml`, `spf13/cobra`, `pterm/pterm`, `go.jonnrb.io/vanity`.

## Load these skills

Default skill set for this project (consumed by `ace.toml`):

- `ace*` — session workflow, save, audit, realign, school
- `general-coding` — per-slice workflow + cross-language conventions
- `go-coding` — Go is the implementation language
- `prod9-fx` — `fx.prodigy9.co v0.4.0` is in `go.mod`
- `cue-coding` — `tests.cue` drives the smoke harness
