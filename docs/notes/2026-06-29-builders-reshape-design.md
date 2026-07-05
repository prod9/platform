# Builders reshape — design pass (#4)

> Status: **design proposal, not implemented.** The reshape touches the
> `builder.Interface` contract and every builder; the strategic fork in §3 is chakrit's
> call. This note exists so we settle the shape *before* adding the platform self-builder
> + infra-render builder, rather than extending a factoring we're about to rework (the
> stated goal of task #4). Grounded in a full factoring map of `builder/` (this session).
> The general-coding audit's dedup findings (GC-D1 ×several, GC-U6, GC-X1) all point at
> the same structural gaps and are deliberately deferred here rather than patched
> piecemeal — see [`2026-06-28-coding-audit.md`](2026-06-28-coding-audit.md).

## 0. Resolution (2026-06-29) — read this first

The §3 fork below is **closed**, and several §4 proposals are overridden by operator
decisions this session. The original §1–§6 are kept as the factoring map + reasoning
record, but where they conflict, this section wins.

### The fork is dissolved: infra-render is a new builder that returns a container

There is **no Option A/B fork.** infra-render becomes a **new builder type** (the
`platform/infra` builder) that returns `*dagger.Container` like every other builder — the
`Interface` output type is unchanged. It is **2-phase**, reusing the existing build→runner
shape:

| Phase    | Go builder        | `platform/infra` builder                         |
|----------|-------------------|--------------------------------------------------|
| build    | source → binary   | CUE source → `gitops`/`ops render` → k8s YAML     |
| run/pack | binary → run image | rendered YAML → `FROM scratch` + the manifests   |

The "runner" output is a **plain OCI image** carrying the rendered manifests. This is
distinct from building the *server image* — building a runnable server and packing an
already-rendered deploy artifact are two separate activities; the infra artifact is a
config bundle that references an already-built image by committed literal, never something
you run.

### Why a plain image works: the Flux consumption side, not the producer side

Dagger **cannot** emit Flux's artifact media types. Its only media-type knob is
`ImageMediaTypes` = `OCIMediaTypes | DockerMediaTypes` (image manifests only); there is no
way to set the config media type (`application/vnd.cncf.flux.config.v1+json`), the flux
content layer type (`…flux.content.v1.tar+gzip`), or an artifact `artifactType`.
`WithAnnotation` adds OCI annotations, not media types. So a dagger-published artifact is
structurally an **image**, not the flux-shaped artifact `gitops.Publish` currently
hand-builds with oras-go.

The resolution moves the compatibility from the **producer** to the **consumer**: Flux's
`OCIRepository` consumes a **plain image** via a `layerSelector` (`mediaType:
application/vnd.oci.image.layer.v1.tar+gzip`, `operation: extract`); the extracted layer's
YAML is applied by **kustomize-controller** (a `Kustomization` over plain manifests — no
overlays authored). This is the canonical, well-supported Flux OCI path and is **already
in production on stage9** (`OCIRepository` + `Kustomization`; see the `gitops` package).
So the new model keeps the same consumer pair and only swaps the artifact shape (plain
image + `layerSelector`) for the flux-media-type artifact — retiring the
oras/flux-media-type path in `gitops.Publish`.

**Reproducibility is deliberately a non-issue.** kustomize-controller's apply is
idempotent server-side: identical content ⇒ no-op apply regardless of digest. A dagger
layer tarball may carry timestamps and re-push a new digest on an unchanged render, but
that only costs a harmless re-pull + re-apply; nothing depends on artifact-level digest
stability here. (Synthesized files via `WithNewFile` tend to get fixed timestamps anyway.)

The **clone** for a CI render is **not** a builder concern — it is the repo-prep phase in
the platform-as-CI design
([`2026-06-29-platform-as-ci-design.md`](2026-06-29-platform-as-ci-design.md)), which
lands a local working tree; the builder renders in-process against it.

### §4 dispositions (operator decisions this session)

| §  | Original proposal              | Decision                                                                 |
|----|--------------------------------|--------------------------------------------------------------------------|
| 1  | Universal `withRunner`         | **Reject** — run stages differ wildly per runner; deliberate non-abstraction. **Open:** `go_basic`/`go_workspace` `// run` blocks are byte-identical (same Go runner) — within-language sharing is unresolved, not the rejected cross-language abstraction. |
| 2  | Resolve cmd/args/outdir on unit | **Accept, revised → two-phase.** The unit holds *declarative* fields (separate container-local paths from build-args, command from args); contextual resolution (`./` → `/app`, entrypoint-vs-args convention, which differs per build) stays at **build time** per builder. Not "computed once at construction" — the unit is defined outside any build context. |
| 3  | Force env/assets/sync uniform  | **No** — make non-support **explicit**, don't unify. Not all features apply to all builders (asset dirs don't fit a Dockerfile). `Dockerfile` stays a deliberate **escape hatch** (leave as-is, or elevate a level later). |
| 4  | Generic workspace discovery    | **Defer** — discovery + local-repo bootstrapping is its own feature set (a marker-file scanner), to design wholesale, not piecemeal-DRY now. |
| 5  | Drop `Layout()`/`Class()`      | **Keep** — latent placeholders for a planned (currently dormant) IO/file-manipulation + bootstrap feature. |
| 6  | Replace order-sensitive registry | **Defer** — ordering only matters during discovery (`platform.toml` requires an explicit `builder=""` otherwise); fold into the discovery redesign. |
| 7  | `fileutil` rename              | **Drop** — premise void: `internal/fileutil` never existed, so there is no collision. `builder/fileutil` (`DetectFile`/`WalkSubdirs`) keeps its name. |
| 8  | Dockerfile build-args bug      | **Already fixed** — landed in `a393a0f` (audit triage). |

Also confirmed: **II.1** platform-builds-itself fits `GoBasic` **as-is, no opt-out needed**.
The "friction" once claimed here — that `GoBasic`'s unconditional `go test -v ./...` needs a
per-unit opt-out — was a **conflation** of the in-build hermetic suite (`go test ./...`, no
docker) with the docker smoke harness (`./test.sh`, host-only). Platform's `go test ./...` is
hermetic and runs fine in a fresh clone, so self-build was never blocked. The opt-out is
**WONTFIX** (same class of error as the `fileutil` "collision", slice 1); test-in-build is a
hard, non-configurable gate. Recorded in the
[test-in-build ADR](../decisions/2026-07-05-test-in-build-is-a-hard-gate.md). **II.2**
infra-render is a *new* builder, not a generalization of the contract (above).

What survives as near-term reshape work: the two-phase cmd/args separation (§2) and making
feature non-support explicit (§3) — then the new `platform/infra` builder. Dropped/deferred:
the `GoBasic` test opt-out (WONTFIX, above), universal run-stage, generic discovery, dropping
`Layout()`/`Class()`, registry reorder, the `fileutil` rename (no collision).

## 1. What's actually wrong (not style — structure)

The package assumes **one shape of output (a runnable container) from one shape of input
(a filesystem-detected source dir)**. Within that assumption it works, but it has no home
for the work every builder shares, so the sharing is copy-paste:

1. **No run-stage abstraction.** The "assemble the runtime image" block (runner pkgs → env
   → copy artifact → copy asset dirs → default args → `Sync`) is re-implemented inline in
   every builder. The Go version is *byte-identical* between `go_basic.go:72-81` and
   `go_workspace.go:125-134`. This is the single largest dedup target.
2. **Stringly-typed cmd/args/outdir resolution scattered across 6 files**, with silent
   divergence: Go builds `"./"+cmd`, PNPM uses bare `cmd` + default arg `"."`, outdir
   default `"build"` is repeated verbatim 3×. The resolution belongs on `BuildUnit` as
   resolved fields, computed once at unit construction — not redone in each `Build`.
3. **Every cross-cutting concern lands in a different place per builder:**
   - `withUnitEnv`: Go on the *runner*, PNPM on the *base*, PNPMStatic *nowhere*,
     Dockerfile on the build result. No owner.
   - `AssetDirs`: honored by 4 of 6; PNPMStatic and Dockerfile silently drop it.
   - `Sync`: 5 builders self-sync; PNPMStatic returns unsynced and relies on the engine
     re-syncing. Two contracts for one method.
4. **Parallel workspace discovery.** `go_workspace` and `pnpm_workspace` `Discover` are
   the same algorithm with a different delegate (`pnpm_workspace.go:36-38` admits the
   copy).
5. **Dead metadata in the Interface.** `Layout()` and `Class()` are required by the
   contract but **read nowhere** in build/discover/engine/cmd. `ClassBytecode` has no
   implementor. They are documentation masquerading as behavior.
6. **Order-sensitive registry** (`builder.go:75-82`): correctness depends on slice
   position (workspace must precede basic), guarded only by a comment.
7. **One real bug rides along:** `dockerfile.go:56` assembles `opts` with `Env` build-args
   then passes a *fresh empty* opts to `DockerBuild` — build-args are silently dropped.

## 2. The crux: the new builders don't fit the current contract

`Interface.Build(ctx, *dagger.Client, *BuildUnit) (*dagger.Container, error)`.

- **platform-builds-itself (a Go binary)** fits `GoBasic` *as-is* — single `go.mod`, no
  work needed. (An earlier draft flagged `GoBasic`'s in-build `go test -v ./...` as needing
  an opt-out; that conflated the hermetic in-build suite with the docker smoke harness —
  **WONTFIX**, see §0 II.1 and the test-in-build ADR. The gate is deliberate.)
- **infra-render (CUE/manifests → OCI artifact)** does **not** fit. It produces an
  artifact, not a runnable container. The closest existing shape is `gitops.Publish`/`ops`
  — already an OCI-artifact path **outside** `builder/`. Forcing it through `Build →
  *dagger.Container` means either wrapping render output in a throwaway container, or
  generalizing the return type — which ripples to `engine.Build`, `BuildResult.Container`,
  and every `cmd/` reader of `result.Container`.

## 3. Strategic fork — chakrit's call

**Does infra-render become a `builder.Interface`, or stay its own pipeline?**

- **Option A — keep `builder.Interface` container-only; infra-render stays in
  `gitops`/`ops`.** The 6 builders all return containers; clean them up *for that* (§4).
  The infra path already lives in `gitops.Render`/`Publish` — extend it, don't fold it in.
  Smallest blast radius; honours the existing `engine → builder → project` graph and the
  gitops OCI path. "platform self-build" is just a Go container build (fits today). The
  render pipeline and the build pipeline stay separate concerns with separate output
  types. *Cost:* "builder" no longer means "everything that produces a deployable
  artifact" — there are two production paths (containers via `builder`, artifacts via
  `gitops`).

- **Option B — generalize the contract to `Build(...) (Artifact, error)`** where
  `Artifact` is a sealed type (`ContainerArtifact | OCIArtifact`). One pipeline, one
  registry, one discovery. *Cost:* ripples through `engine.Build`, `BuildResult`, and
  every `cmd/` consumer; `Discover`'s `map[string]Interface` (filesystem-keyed) doesn't
  fit a config-keyed infra builder without also reworking discovery. Bigger, but unifies
  "how do I produce a thing to ship."

**Recommendation: Option A.** The map is clear that infra-render is shaped like `gitops`,
not like the builders, and B's generalization buys unification we don't yet need at the
cost of churning the engine/cmd boundary. A also lets the §4 cleanup proceed immediately
without blocking on the output-type question. Revisit B only if a third artifact-producing
case appears that *also* wants file-detection + the build engine.

Everything in §4 is **independent of this fork** — it's the container-builder cleanup,
valid under either option.

## 4. Proposed factoring (container builders)

1. **Resolve cmd/args/outdir once, onto `BuildUnit`.** Add resolved fields (`Command`,
   `Args`, `OutDir`) computed in `unitFromModule`/`AttemptFrom`, with per-class defaults
   applied there. `Build` reads resolved fields; the scattered switches (GC-D1) disappear.
2. **A real run-stage.** Extract `withRunner(base, unit, artifacts, args)
   *dagger.Container` (or a small `RunSpec` value) owning: runner pkgs, env, artifact
   copy, asset-dir copy, default args, `Sync`. Every builder ends with one call. Kills the
   byte-identical copy-paste and forces env/assets/sync to have exactly one owner (fixes
   the §1.3 drift, GC-X1).
3. **Generic workspace discovery.** `discoverWorkspace(wd, basic, self Interface)`
   collapses the two parallel `Discover` bodies (§1.4).
4. **Drop dead metadata.** Remove `Layout()` and `Class()` from the Interface (nothing
   reads them). If discovery ordering needs to be explicit, replace the order-sensitive
   registry (§1.6) with a declared priority on each builder, or have workspace builders'
   `Discover` subsume their basic counterpart explicitly.
5. **Proposed Interface:**
   ```go
   type Interface interface {
       Name() string
       Discover(wd string) (map[string]Interface, error)
       Build(ctx context.Context, client *dagger.Client, unit *BuildUnit) (*dagger.Container, error)
   }
   ```
(Same as today minus `Layout()`/`Class()`. Output type unchanged under Option A.)
6. **Ride-along fixes** folded into the reshape: `dockerfile.go` build-args bug (§1.7);
   PNPMStatic single base pull + honor env/assets. (The `GoBasic` test opt-out once listed
   here is WONTFIX — see §0 II.1.)
7. **Tag application (GC-U6).** `cmd/publish.go:66` + `deploy.go:92` inline `ImageName +=
   ":"+tag` — move onto the attempt/unit so one place owns it; both commands call it.

## 5. Suggested slice order (once the fork is decided)

1. Resolve cmd/args/outdir onto `BuildUnit` (no behavior change; smoke stays UNCHANGED).
2. Extract `withRunner`; convert builders one at a time. ← biggest win, normalizes
   env/assets/sync.
3. Generic `discoverWorkspace`.
4. Drop `Layout()`/`Class()`; fix registry ordering.
5. Ride-along fixes (dockerfile args, tag-on-attempt). [GoBasic test opt-out: WONTFIX, §0 II.1]
6. Then add the platform self-builder (trivial) and — per the fork — the infra-render
   path.

Each slice is behavior-preserving except the explicit bug fixes; the smoke harness is the
drift gate at each step.

## 6. Open decisions (blockers for chakrit)

- **§3 fork:** Option A (builder stays container-only, infra-render in gitops) vs B
  (generalize output type). Recommend A. *Everything else can proceed regardless.*
- **Drop `Layout()`/`Class()`?** ~~They're dead today.~~ **Resolved for `Class` (slice 4,
  2026-07-05): kept and repurposed as the runtime-shape taxonomy, not dropped.** `Class`
  describes what the *runner* is — native binary / bytecode+VM / interpreted process /
  static-served / custom — independent of the build toolchain. Added `ClassStatic` and moved
  `PNPMStatic` onto it (it was mis-labelled `ClassInterpreted`, whose own doc requires the
  build tooling at runtime — false for a caddy-served bundle). Still descriptive-only (nothing
  switches on it yet), but now a correct axis to switch on later. `Layout` still dead.

  **Deferred larger rethink (chakrit's, 2026-07-05):** split the two axes the builders
  currently fuse — a **runtime-shape** axis (`Class`: static-binary / static-served / …) and a
  **language/framework metadata** axis (go, pnpm/node, …) — toward *one builder per class* with
  language carried as metadata (e.g. a single static-binary builder + Go metadata, rather than
  `GoBasic`/`GoWorkspace` each re-encoding "produces a native binary"). `ClassStatic` is the
  first correct instance of that shape. Full split is a much larger refactor — not now.
- **Registry ordering:** replace the implicit slice-order contract with explicit priority,
  or keep the comment-guarded order?
