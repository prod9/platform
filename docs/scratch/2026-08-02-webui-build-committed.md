<!-- not spec/decision because: no ruling made; the alternative is blocked on a feature
that does not exist yet -->

# Why `webui/build/` is committed

`webui/webui.go` embeds the SvelteKit output with `//go:embed all:build`, so `build/` is
tracked in git and rebuilt by hand with `pnpm build`. The bundle the binary ships is
whatever the last commit put there.

## What keeps it that way

`go:embed` is resolved by the compiler. If `build/` were generated rather than committed,
every path that compiles the `webui` package would need `pnpm build` to have run first —
`go build`, `go run .`, `go test ./...`, and the container's own `StepTest`, which runs
before `StepBuild` in the Go plan. A fresh clone would fail to compile until someone
remembered the generate step.

Nothing in the Go toolchain closes that gap. `go build` and `go test` never invoke
`go generate`, so a `//go:generate` directive would name the command without running it.

## What would change it

A build hook that runs before the Go build — the `BeforeBuild` point sketched in the
[test-in-build ADR](../decisions/2026-07-05-test-in-build-is-a-hard-gate.md), where opt-in
hooks are the sanctioned path for per-project setup. With one, `pnpm build` becomes part of
the build rather than a precondition on it, and `build/` can leave git.

Hooks are unbuilt and unspecified. This file records the use case for whoever designs them;
it is not a commitment that hooks will be shaped around it.
