# Builders reshape — design pass (#4)

> Status: **design proposal, not implemented.** The reshape touches the `builder.Interface`
> contract and every builder; the strategic fork in §3 is chakrit's call. This note exists so
> we settle the shape *before* adding the platform self-builder + infra-render builder, rather
> than extending a factoring we're about to rework (the stated goal of task #4).
>
> Grounded in a full factoring map of `builder/` (this session). The general-coding audit's
> dedup findings (GC-D1 ×several, GC-U6, GC-X1) all point at the same structural gaps and are
> deliberately deferred here rather than patched piecemeal — see
> [`2026-06-28-coding-audit.md`](2026-06-28-coding-audit.md).

## 1. What's actually wrong (not style — structure)

The package assumes **one shape of output (a runnable container) from one shape of input (a
filesystem-detected source dir)**. Within that assumption it works, but it has no home for the
work every builder shares, so the sharing is copy-paste:

1. **No run-stage abstraction.** The "assemble the runtime image" block (runner pkgs → env →
   copy artifact → copy asset dirs → default args → `Sync`) is re-implemented inline in every
   builder. The Go version is *byte-identical* between `go_basic.go:72-81` and
   `go_workspace.go:125-134`. This is the single largest dedup target.
2. **Stringly-typed cmd/args/outdir resolution scattered across 6 files**, with silent
   divergence: Go builds `"./"+cmd`, PNPM uses bare `cmd` + default arg `"."`, outdir default
   `"build"` is repeated verbatim 3×. The resolution belongs on `BuildUnit` as resolved fields,
   computed once at unit construction — not redone in each `Build`.
3. **Every cross-cutting concern lands in a different place per builder:**
   - `withUnitEnv`: Go on the *runner*, PNPM on the *base*, PNPMStatic *nowhere*, Dockerfile on
     the build result. No owner.
   - `AssetDirs`: honored by 4 of 6; PNPMStatic and Dockerfile silently drop it.
   - `Sync`: 5 builders self-sync; PNPMStatic returns unsynced and relies on the engine
     re-syncing. Two contracts for one method.
4. **Parallel workspace discovery.** `go_workspace` and `pnpm_workspace` `Discover` are the same
   algorithm with a different delegate (`pnpm_workspace.go:36-38` admits the copy).
5. **Dead metadata in the Interface.** `Layout()` and `Class()` are required by the contract but
   **read nowhere** in build/discover/engine/cmd. `ClassBytecode` has no implementor. They are
   documentation masquerading as behavior.
6. **Order-sensitive registry** (`builder.go:75-82`): correctness depends on slice position
   (workspace must precede basic), guarded only by a comment.
7. **Two `fileutil` packages** (`builder/fileutil` vs `internal/fileutil`) — a name collision.
8. **One real bug rides along:** `dockerfile.go:56` assembles `opts` with `Env` build-args then
   passes a *fresh empty* opts to `DockerBuild` — build-args are silently dropped.

## 2. The crux: the new builders don't fit the current contract

`Interface.Build(ctx, *dagger.Client, *BuildUnit) (*dagger.Container, error)`.

- **platform-builds-itself (a Go binary)** fits `GoBasic` *as-is* — single `go.mod`, no work
  needed. The only friction: `GoBasic` runs `go test -v ./...` unconditionally inside the build
  (`go_basic.go:69`), so a slow/failing test blocks the image. Needs an opt-out on the unit.
- **infra-render (CUE/manifests → OCI artifact)** does **not** fit. It produces an artifact,
  not a runnable container. The closest existing shape is `gitops.Publish`/`ops` — already an
  OCI-artifact path **outside** `builder/`. Forcing it through `Build → *dagger.Container` means
  either wrapping render output in a throwaway container, or generalizing the return type — which
  ripples to `engine.Build`, `BuildResult.Container`, and every `cmd/` reader of `result.Container`.

## 3. Strategic fork — chakrit's call

**Does infra-render become a `builder.Interface`, or stay its own pipeline?**

- **Option A — keep `builder.Interface` container-only; infra-render stays in `gitops`/`ops`.**
  The 6 builders all return containers; clean them up *for that* (§4). The infra path already
  lives in `gitops.Render`/`Publish` — extend it, don't fold it in. Smallest blast radius;
  honours the existing `engine → builder → project` graph and the gitops OCI path. "platform
  self-build" is just a Go container build (fits today). The render pipeline and the build
  pipeline stay separate concerns with separate output types.
  *Cost:* "builder" no longer means "everything that produces a deployable artifact" — there
  are two production paths (containers via `builder`, artifacts via `gitops`).

- **Option B — generalize the contract to `Build(...) (Artifact, error)`** where `Artifact` is a
  sealed type (`ContainerArtifact | OCIArtifact`). One pipeline, one registry, one discovery.
  *Cost:* ripples through `engine.Build`, `BuildResult`, and every `cmd/` consumer; `Discover`'s
  `map[string]Interface` (filesystem-keyed) doesn't fit a config-keyed infra builder without
  also reworking discovery. Bigger, but unifies "how do I produce a thing to ship."

**Recommendation: Option A.** The map is clear that infra-render is shaped like `gitops`, not
like the builders, and B's generalization buys unification we don't yet need at the cost of
churning the engine/cmd boundary. A also lets the §4 cleanup proceed immediately without
blocking on the output-type question. Revisit B only if a third artifact-producing case appears
that *also* wants file-detection + the build engine.

Everything in §4 is **independent of this fork** — it's the container-builder cleanup, valid
under either option.

## 4. Proposed factoring (container builders)

1. **Resolve cmd/args/outdir once, onto `BuildUnit`.** Add resolved fields (`Command`, `Args`,
   `OutDir`) computed in `unitFromModule`/`AttemptFrom`, with per-class defaults applied there.
   `Build` reads resolved fields; the scattered switches (GC-D1) disappear.
2. **A real run-stage.** Extract `withRunner(base, unit, artifacts, args) *dagger.Container` (or
   a small `RunSpec` value) owning: runner pkgs, env, artifact copy, asset-dir copy, default
   args, `Sync`. Every builder ends with one call. Kills the byte-identical copy-paste and
   forces env/assets/sync to have exactly one owner (fixes the §1.3 drift, GC-X1).
3. **Generic workspace discovery.** `discoverWorkspace(wd, basic, self Interface)` collapses the
   two parallel `Discover` bodies (§1.4).
4. **Drop dead metadata.** Remove `Layout()` and `Class()` from the Interface (nothing reads
   them). If discovery ordering needs to be explicit, replace the order-sensitive registry
   (§1.6) with a declared priority on each builder, or have workspace builders' `Discover`
   subsume their basic counterpart explicitly.
5. **Proposed Interface:**
   ```go
   type Interface interface {
       Name() string
       Discover(wd string) (map[string]Interface, error)
       Build(ctx context.Context, client *dagger.Client, unit *BuildUnit) (*dagger.Container, error)
   }
   ```
   (Same as today minus `Layout()`/`Class()`. Output type unchanged under Option A.)
6. **Ride-along fixes** folded into the reshape: `dockerfile.go` build-args bug (§1.8); rename
   `builder/fileutil` to a responsibility name (e.g. `dirscan`) to end the collision (§1.7);
   `GoBasic` test-step opt-out on the unit (§2); PNPMStatic single base pull + honor env/assets.
7. **Tag application (GC-U6).** `cmd/publish.go:66` + `deploy.go:92` inline `ImageName += ":"+tag`
   — move onto the attempt/unit so one place owns it; both commands call it.

## 5. Suggested slice order (once the fork is decided)

1. Resolve cmd/args/outdir onto `BuildUnit` (no behavior change; smoke stays UNCHANGED).
2. Extract `withRunner`; convert builders one at a time. ← biggest win, normalizes env/assets/sync.
3. Generic `discoverWorkspace`.
4. Drop `Layout()`/`Class()`; fix registry ordering.
5. Ride-along fixes (dockerfile args, fileutil rename, GoBasic test opt-out, tag-on-attempt).
6. Then add the platform self-builder (trivial) and — per the fork — the infra-render path.

Each slice is behavior-preserving except the explicit bug fixes; the smoke harness is the drift
gate at each step.

## 6. Open decisions (blockers for chakrit)

- **§3 fork:** Option A (builder stays container-only, infra-render in gitops) vs B (generalize
  output type). Recommend A. *Everything else can proceed regardless.*
- **Drop `Layout()`/`Class()`?** They're dead today. Keep only if they're meant to drive
  near-term behavior (display grouping, discovery priority) — say which.
- **Registry ordering:** replace the implicit slice-order contract with explicit priority, or
  keep the comment-guarded order?
