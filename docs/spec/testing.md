# Testing — the two suites and the drift detector

Status: **implemented.** The Law that governs *when* these run (blackbox-first;
test-in-build is a hard gate; runs never gate on the operator) lives in `CLAUDE.md`. This
spec owns the mechanism.

## Two suites, at different layers

- **`go test ./...`** — hermetic unit tests (no docker/network, fresh-clone runnable). Runs
  inside **every image build** (the `Go*` framework gate — see
  [`frameworks.md`](frameworks.md)) and locally on demand. The light complement, not the
  primary strategy.
- **`./test.sh`** — blackbox smoke (`chakrit/smoke`): drives the built binary through Dagger
  against the testbeds; **needs docker**. Runs on the host, manually / pre-publish.

`./test.sh` runs `cue eval tests.cue → tests.yml` → the `chakrit/smoke` runner. Tests build
the binary, then for each testbed run `init`/`build` checking exitcode/stdout/expected
files. `./testbed.sh <dir> <args>` runs platform inside a specific testbed. `testbeds/`
holds one sample project per framework.

## Smoke is a drift detector, not an assertion engine

`tests.lock.yml` is a recorded golden of each command's *actual* output — exitcode, stdout,
and the content of any non-reserved `checks:` entry (a file glob snapshots its matched
files' bytes, not just their existence). The golden is whatever the command last produced,
not a hand-authored "correct" value: correctness is established once, by a human reviewing
the diff when a line is recorded; thereafter the test guards only against *unreviewed
change*.

So a green run (`UNCHANGED`) means "nothing drifted," not "behavior is correct." A red run
(`CHANGED`, exit 1) means output moved off the golden — a prompt to **review the diff and
decide**, not a failed assertion.

- Intended drift → re-record with `./test.sh --commit`.
- Unintended → a regression to fix at the source.
- Never `--commit` a CHANGED lock unread, and never massage code just to force output back
  onto the old golden — both blind the detector.
- A slice that moves smoke output isn't done until its golden is re-recorded and the diff
  reviewed, **in that same slice** — same tier as `go test` passing, never a session-end
  batch. The docker/runtime cost is mechanism, not a deferral.

## The 1m per-test timeout

The per-test timeout in `tests.cue` is deliberately tight — it keeps builds honest. Never
raise it to make a slow build pass: fix the slowness (cache reuse, unnecessary work,
network pulls) instead, since a slowdown landed by one person taxes everyone's local and CI
cycles. Cold-cache pulls of a freshly pinned image are the one accepted cause — verify by
warming the cache and re-running, not by touching the timeout.

## Gotchas

- **Stale testbed files.** A testbed with no committed `platform.toml` accumulates
  gitignored leftovers, which make `init` *merge* rather than *generate*. Such tests wipe
  the target first (e.g. infra-init).
- **Non-interactive init.** Value prompts read positional args; `ALWAYS_YES=1` only
  auto-answers the final yes/no confirm. See `CLAUDE.md` and the `tests.cue` init
  invocations.
