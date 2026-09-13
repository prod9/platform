<!-- not spec/decision because: bounded verification checkpoint -->

# Module jobs: stop at Phase 2

Audience: chakrit reviewing the retained candidate and an engineer verifying that scope.

The authorized task is to retain admission and execution through Phase 2, discard the
uncommitted Phase 3 runtime policy implementation, compile and test the result, then stop.
There is no next implementation phase authorized by this checkpoint.

## Retained candidate

- Immutable manifest observations shared by registration and build admission.
- Exact-commit manual and webhook admission, selected module rows, and operator retry.
- Module-based detail, feed, and step reads replacing attempts.
- Dispatcher, guarded module jobs, paired lifecycle events, and complete result folds.
- Engine source/configuration ownership, session cleanup, and all CLI Observer consumers.
- Per-module registry credentials and tag-only server publication: branches only build;
  tag refs publish under the exact tag name.
- Supporting tests, removed obsolete attempt helper, and its generated frontend assets.

The worker no longer reads stored publish policy or selects always/tags/never behavior.
The policy remains part of the immutable manifest observation; its parser and persisted
shape predate the discarded execution work. The current candidate contract is recorded in
[platform-server.md](../spec/platform-server.md), [engine.md](../spec/engine.md), and
[execution-modes.md](../spec/execution-modes.md).

## Verification and stop

Run `go test ./...` with database tests enabled against the existing local PostgreSQL,
`pnpm test` in `webui`, and `./test.sh`. Review smoke drift and re-record only intended
output. Preserve timeout budgets and published migration files. Inspect the complete
retained diff against the Phase 2 contract before closing.

Verification completed on 2026-09-13: Go tests passed with PostgreSQL tests enabled,
all 88 frontend tests passed, and the final smoke comparison was UNCHANGED after the
reviewed missing-module golden addition. Full retained-scope audits required no further
changes. Evidence is captured in `tmp/phase2.local/`.
Stop after this candidate's verification and close: no release, push, deployment,
publication-policy rollout, or other backlog work follows automatically.
