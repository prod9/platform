# Test-in-build is a hard gate; blackbox-first testing

- **Date:** 2026-07-05
- **PR:** manual
- **Status:** accepted

## Decision

Green tests are a **baked-in, non-configurable** precondition of every image build. The
`Go*` builders run `go test ./...` inside the build (`go_basic.go`, `go_workspace.go`); red
tests fail the build and no image is produced. There is **no skip-tests opt-out** — not a
`platform.toml` key, not a flag. Building an image from red tests is a non-use-case.

Testing strategy is **blackbox-first**: prefer the `./test.sh` smoke harness (blackbox —
drives the built binary through Dagger) over many small unit tests, on ROI grounds. This is
why platform historically carried almost no `go test`s. Unit tests are the light hermetic
complement, not the primary strategy.

Two suites run at two layers:

- **`go test ./...`** — hermetic unit tests: no docker, no network, runnable in a fresh dev
  clone. Runs **inside every image build** (the gate) and locally on demand.
- **`./test.sh`** — blackbox smoke (`chakrit/smoke`): needs docker. Runs **on the host**,
  manually / pre-publish — the drift detector.

Tests needing infra beyond the hermetic default (docker, a DB, an e2e harness) must be
build-tagged out of the default `go test ./...` so they never leak into the in-build gate;
they run only under explicit opt-in.

## Rationale

Enforcement is deliberately **local and in-process**, not spread across CI phases: the build
itself is the gate, so there is one hard-coded, opinionated flow rather than a pipeline of
optional stages a project can rearrange or skip. An image built from red tests has no
consumer, so gating costs nothing real and removes a whole class of "shipped broken" states
at the source. Sibling stance: the
[opinionated-appliance ADR](2026-06-17-opinionated-appliance-embedded-init.md).

The blackbox-first bias is an ROI judgment: tests earn the most at the boundary (the built
binary against real testbeds) and the least as a swarm of small unit tests coupled to
internals. `./test.sh` is that boundary suite; `go test` fills hermetic gaps unit tests are
genuinely good at.

**The conflation this ADR closes.** The 2026-06-29 builders-reshape design note (§2, §4.6,
§6) claimed `GoBasic` needs a per-unit *test opt-out* as a "prerequisite for platform
self-build," reasoning that a slow or failing in-build test would block the image. That
conflated the two suites: the in-build `go test ./...` is hermetic (verified — no test in
the tree imports `dagger.io/dagger`, opens a socket, or does network I/O; `dsl/io_test.go`
injects a `Fetch` mock, `gitops/publish_test.go` uses an in-memory oras store), while the
docker-requiring suite is `./test.sh`, which runs on the host and **never inside the build**.
Platform's `go test ./...` runs fine in a fresh clone without docker, so self-build was never
blocked. Same class of error as the dropped `fileutil` "collision" (reshape slice 1): a
note-level assumption ungrounded in the code.

## Consequence

- The `GoBasic` test opt-out (design-note §2 / §4.6 / §6, reshape "slice 3") is **WONTFIX**
  — it contradicts the gate and rests on the conflation above.
- Reshape slice 5 (platform self-build) is **unblocked**: `GoBasic` fits platform as-is.
- The sanctioned path for heavier per-project setup is **opt-in build hooks**
  (`BeforeTest` / `AfterTest` / `BeforeBuild` / …) — additive setup, never a gate opt-out.
  Logged as a feature, not yet built.
- The behavior-changing FHS work of the same date is unrelated; see the
  [FHS container-layout ADR](2026-07-05-platform-fhs-container-layout.md).
