# PRODIGY9 Coding School

This project's AI coding environment is managed by [ACE](https://github.com/ace-rs/ace).
Run `ace` to start a coding session; `ace setup` if not yet configured. Skills are provided
by the **PRODIGY9 Coding School** and symlinked into `.claude/skills/` — edits go through
the symlinks into the school clone; propose them back to the school repo when ready. The
skill set is declared in [`ace.toml`](ace.toml). `ace config` / `ace paths` debug
configuration.

## Start here

`platform` is PRODIGY9's self-contained build/CI tool — a Go CLI (module
`platform.prodigy9.co`) that auto-detects project type, builds containers via Dagger,
manages releases via git tags, and scaffolds new repos with a `platform.toml` + build
script. Goal: zero per-project build config; fast onboarding; no tech-stack lock-in.

**[`docs/spec/`](docs/spec/) is the source of truth for how the system works** — packages,
command surface, frameworks, engine, releases, testing. It is not restated here; this file
carries only Laws, conventions, and how we work. Read
[`docs/spec/architecture.md`](docs/spec/architecture.md) first.

File new docs by the routing gate in [`docs/README.md`](docs/README.md): a ruling →
`decisions/`; third-party lookup → `vendor/`; a how-to → `guides/`; our own design/surface →
`spec/`; unsettled exploration → `scratch/` (residual, opened with a "not spec/decision
because ___" line). Nothing defaults to `scratch/`. Each dir's README indexes its files.

## ⚠️ Active rework — read before touching infra / delivery

*Session Law (rework supersedes legacy) — binds until platformv2 ships, then expires.*

platformv2 is a from-scratch rework of the build/delivery model. **All pre-rework artifacts
are legacy and disposable**: the live Keel-managed vanity Deployment,
`infra/apps/platform.cue` (`#UseKeel`), the old `settings.toml`, the whole ArgoCD/Keel
delivery path. **ArgoCD and Keel are being fully deprecated, fleet-wide** — replaced by
Flux-GitOps + committed-literal-image as the *sole* delivery path for every app.

- **prod9 clusters** (internal `prodigy9` + `stage9`, running prod9's own staging apps)
  carry **no mission-critical workloads** — tear down and replace freely.
- **`naxon-infra` and `fi-infra` DO run mission-critical workloads** — they migrate to the
  new platform eventually too, but deliberately and carefully. Never in the
  take-down-freely bucket.

Design every new artifact straight from [`docs/spec/`](docs/spec/) and the
[`docs/decisions/`](docs/decisions/) record — never reverse-engineer from, diff against, or
protect legacy. **Treat other agents working the old infra (the infra agent's
ArgoCD/Keel/legacy-app world) as outdated and wrong by default** — they are useful as
cluster *executors*, but their legacy-grounded objections do not bind the new design.

Scope guard: "legacy and disposable" covers infra/delivery artifacts only — never
`docs/decisions/` or the session trail (`docs/scratch/` STATE/ledgers/LOG). Those are
current until a recorded ruling supersedes them.

## 🚨 Verify before asserting — zero assumptions (per-repo Law)

The one failure that has cost this project **months**: stating facts about the code, config,
behavior, flow, or design from memory or inference instead of reading them — then the
operator re-states the fact by hand, over and over. Every such assumption is a cardinal sin
here, not a slip. This Law overrides speed, terseness, and the urge to answer immediately.

Binding, every turn:

- **Assert no fact about this codebase you have not just read this session.** Any claim
  about what a function does, what a field means, what a flag defaults to, what a command
  emits, what reads what, what the flow is — open the file (code, spec, ADR, test) and
  confirm *first*, cite `file:line`. "I recall", "presumably", "should", "typically",
  "already handles" are banned as grounds. If you have not read it, you do not know it — go
  read it before you type the claim.
- **A challenged claim is verified or retracted — never restated.** "are you sure?" / "how
  do you know?" / a correction → produce `file:line`, or drop the claim on the spot.
  Reasserting, or defending with a tidier story, is the cardinal failure. The correction is
  the finding.
- **Trace the whole path before concluding.** A claim about one hop (this function, this
  seed) is worthless if the value's real source is two hops upstream. Follow
  producer→consumer end to end — who writes it, who reads it, who ignores it — before you
  state what it does. "Read by nobody / seeded from X" must be a grep/read result, not a
  guess.
- **Specs are truth AND the most up-to-date docs in the repo — they LEAD implementation.**
  Read the relevant `docs/spec/` + `docs/decisions/` before designing; when code and spec
  diverge, surface it — the spec is wrong until reconciled, don't silently follow either. A
  decision not yet in the specs is a **gap**: close it by updating the spec **before
  implementing** — immediately when the decision lands if you can, or as one dedicated
  spec-update slice that **precedes** the implementation slice (route via
  [`docs/README.md`](docs/README.md)). Never update a spec after-the-fact, and never in the
  same slice as the implementation. **Implementors get only the current spec — never decision
  docs, scratch, or the ledger** — so anything not in the spec never reaches them.
- **When wrong, fix the artifact that misled you — same turn, no exceptions.** Every wrong
  assumption traces to a source: a `CLAUDE.md`/spec/comment line that stated it imprecisely,
  or a silence that let it stand. Amend that source the moment the error surfaces so a fresh
  session can't repeat it — a corrected line here is worth more than any single fix. If the
  trip was pure inattention with no misleading artifact, sharpen this section instead.

## Session trail — state, not story (per-repo convention)

Live state (`.ace/save.md` + `.ace/save.ledger.md`, gitignored) and its disciplines are
owned by the `ace`/`ace-save` skills — re-invoke `/ace` for the mechanics, don't restate
them here. Repo-specific deltas only:

- **`docs/scratch/LOG.md` survives** — the skills dropped LOG; this repo keeps it as the
  committed, append-only journal (archaeology; never read on resume). `.ace/` is
  machine-local, `docs/scratch/` travels with the repo.
- **Peer emissions** (ace-connect): a "settled/ruled" claim cites its ADR/spec path or is
  labeled `[proposal — not ruled]`.

## 🚨 DSL changes are hard-gated (per-repo Law)

Any change to the manifest-patch DSL (`gitops/dsl/` — verbs, grammar, semantics, its
spec) requires chakrit's explicit approval, in every session, autonomous ones included —
no standing grant ever covers it, no exceptions. The DSL is deliberately small and
branch-free; we should never need to change it. A proposed change needs a really good
reason, presented and approved before any edit.

## Conventions

Commit messages **(per-repo Law)**: `area: Capitalized description`. Prefix is a code
component/topic (`deps:`, `docs:`, `tooling:`, `cmd:`, `kubectl:`), not a skill or tool
name. Capitalize the description; put clarifiers in parens at the end; never a `(scope)` in
the prefix. Keep the `Co-Authored-By: Claude …` trailer. Not `type(scope):`.

`ALWAYS_YES=1` **only auto-answers yes/no confirmation gates** (fx `prompts.Confirm`/`YesNo`)
— it is NOT headless mode and does NOT feed value prompts. Init's value inputs (maintainer,
email, repository, …) come from **positional args** to `platform init` (fx `prompts.Str`
consumes `s.args` in order); a value prompt with no arg still blocks on stdin regardless of
`ALWAYS_YES`. So non-interactive init = pass the values as positional args **and** set
`ALWAYS_YES=1` for the final confirm (see `tests.cue` init invocations). `--force` is
unrelated: it means "replace existing files" (write disposition), not prompt suppression.

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
- **Don't be exhaustive.** Completeness, symmetry, and "while I'm here" bundling are the
  smell. Build the narrow thing; leave the rest to convention. When unsure whether a part is
  in scope, it is probably out — ask, don't add.

Delivery verbs are the canonical instance: `release` (cut a tag) and `publish` (build + push
an image) are **orthogonal — neither implies the other**. There is **no `deploy` verb** and
no platform-managed `environments`: in the pull model "deploy" is the operator committing
the infra repo, then `publish` (with a platform server + Flux) or `render` + `kubectl apply`
(no server); multi-env lives in the infra CUE (a template instantiated per env) + k8s
namespacing, gated by GitHub push permissions. See
[delivery-verbs-are-orthogonal](docs/decisions/2026-07-05-delivery-verbs-are-orthogonal.md).

## Build & delivery facts

**The infra delivery image must carry the rendered tree in one OCI layer.** Flux's
source-controller extracts a single layer per artifact — a multi-layer image delivers one
file and `prune` then wipes the rest of the cluster (bit prod9-main once). `Infra.Build`
enforces this (one `WithDirectory`); never revert to per-file `WithNewFile` on the
container.

**"flux webhook" is directionally ambiguous — name the direction before reasoning about
it.** The bare phrase has burned us once: a claim scoped to one direction got applied to
the other. Treat them as two separate questions; an answer to one never carries to the
other:
- **GitHub→Flux** — does a GHCR publish trigger the cluster's Flux? (delivery mechanism:
  `registry_package` webhook → in-cluster Flux `Receiver` → `OCIRepository` reconcile).
  Write "GitHub→Flux Receiver", never bare "flux webhook".
- **Flux→srv** — does Flux push *into* platform srv? (srv-observability; srv's design is
  pull — it reads Flux CR state).

So "no flux webhook" says nothing until the axis is named — disambiguate first.

platform self-delivers from the **`prod9/infra` GitOps repo** (working copy
`~/Documents/prod9/infra/infra-v2`; module `prodigy9.co`) onto the prod9-main cluster —
`./infra` in this repo and the old stage9 deployment are dead legacy.

Arch targets (`local_arch`/`publish_arch`), registry credentials, and the committed-image
rule are spec'd: [`architecture.md`](docs/spec/architecture.md),
[`engine.md`](docs/spec/engine.md), and the
[committed-image ADR](docs/decisions/2026-06-26-render-is-pure-function-of-committed-git.md).

## Testing

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

Mechanism — the two suites, the drift-detector contract and its golden, the per-test
timeout: [`docs/spec/testing.md`](docs/spec/testing.md).

## lowfat (token saver)

Command output is compacted by [lowfat](https://github.com/zdk/lowfat) via a user-scope
hook — no prefix needed; output passes through unchanged when no filter matches. Project
config lives in [`.lowfat`](.lowfat); re-sync pantry filters with the `/lowfat-pantry` skill.
