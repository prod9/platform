<!-- not spec/decision because: a review sheet for one audit pass — delete once reviewed;
the durable output is the commits and docs/guides/go-coding-laws.md -->

# Audit changes — 2026-07-21

Codebase-wide audit against every law in `general-coding`, `go-coding`,
[`go-coding-laws.md`](../guides/go-coding-laws.md), and `CLAUDE.md`'s design section. One
bucket: violations. No borderline.

## Scope

| Auditor | Directories                                          | Status  |
|---------|------------------------------------------------------|---------|
| 1       | `framework/`, `conf/`, `cuemod/`                      | running |
| 2       | `srv/`                                                | running |
| 3       | `cmd/`, `gitops/` (incl. `dsl/`)                      | running |
| 4       | `releases/`, `engine/`, `git/`, `internal/`, `webui/` | running |

## Findings and fixes

### `framework/`, `conf/`, `cuemod/` — 25 violations

| #  | Where                              | Violation                                                    | Fix                                             | Done |
|----|------------------------------------|--------------------------------------------------------------|-------------------------------------------------|------|
| 1  | `pnpm_shared.go:12`                | `NodeVersion`/`PNPMVersion` restate the built repo's own file | read from the repo; delete both                 | **BLOCKED** |
| 2  | `conf/conf.go:67`                  | `Module.GoVersion` read by nothing; silently ignored          | delete the field                                |      |
| 3  | `gowork/gowork.go:11`              | `ErrBadGoWork` never returned; parser has no failure mode     | return it, or delete                            |      |
| 4  | `conf/conf.go:33,56,130-158`       | `Model.Platform`/`Module.Builder` deprecated shims            | delete fields + fold-in blocks                  |      |
| 5  | `framework/gowork/`                | package named for `go.work` but parses `go.mod` too           | rename to what it extracts                      |      |
| 6  | `gowork/gowork.go:24`              | `ParseString` has no non-test caller; `ParseReader` one caller| fold into `ParseFile`; tests drive `ParseFile`  |      |
| 7  | `skel/skel.go:58`                  | `Dest` exported, sole caller same file                        | inline into `Files`                             |      |
| 8  | `framework/base.go:87`             | `withCaddyServer` one-line one-caller wrapper                 | inline                                          |      |
| 9  | `pnpm_shared.go:31`                | `NInstallScript` exported, consumed 8 lines below             | inline                                          |      |
| 10 | `dagger.go` + `version.go`         | two one-function files; helpers exist only for tests          | one build-info file on a `readBuildInfo` seam   |      |
| 11 | `conf/generate.go:43`              | second return can only ever be "all appended"                 | drop the `[]VarChange` return                   |      |
| 12 | `conf/conf.go:73-100`              | exported mutable `*struct` globals, aliased `Excludes`        | `defaultModel()`/`defaultModule()` by value     |      |
| 13 | `conf/resolve_path.go:12`          | CLI flag mutates `conf` package state                         | const + pass the override                       |      |
| 14 | `scaffold/scaffold.go:52`          | `map[string]any` where every value is a `string`              | `map[string]string`                             |      |
| 15 | `scaffold/scaffold.go:32`          | `Spec.Module *conf.Module` — nil as "no module"               | value + discriminator                           |      |
| 16 | `framework.go:22,61,112`           | `Layout` type + method exist for one `if`                     | delete; workspaces set their own `WorkDir`      |      |
| 17 | `framework_test.go:47`             | round-trip test restates the linear scan it tests             | delete or test the normalization boundary       |      |
| 18 | `framework/base.go:36`             | `CacheBuster` hand-synced to a digest 2 lines above           | derive it                                       |      |
| 19 | `platform_infra.go:154`            | two consts read once each; changelog prose above them         | move into `infraVars`; drop the prose           |      |
| 20 | `conf/conf.go:169-208`             | exported entry points buried among private steps              | reorder                                         |      |
| 21 | `framework/attempt.go:38`          | duplicated branch bodies, one ungrouped wall                  | three acts, one accumulator                     |      |
| 22 | `conf/resolve_path.go`             | one-function file; returns a provably-nil err; restating comment | fold into `conf.go`                          |      |
| 23 | `cuemod/cuemod.go:17`              | exported mutable `var` for a fixed path                       | `const`                                         |      |
| 24 | five `*_build` methods             | `// prepare job parameters` / `// build` / `// run` labels    | delete; blank lines already mark the phases     |      |
| 25 | `gowork/gowork_test.go:9`          | exported test fixtures                                        | unexport                                        |      |

### `srv/` — 26 violations

| #  | Where                          | Violation                                                   | Fix                                          | Done |
|----|--------------------------------|-------------------------------------------------------------|----------------------------------------------|------|
| 26 | `github/tokens.go:21-118`      | whole installation-token path dead; 86 test lines serve it   | delete all but `RespError`                   |      |
| 27 | `auth/auth.go:440-468`         | `LoadUserGitHubToken`/`ErrNoGitHubToken` dead, zero callers  | delete; drop the token it encrypts unread    |      |
| 28 | `github/app.go:24`             | `App.Slug` written, read only by a test asserting `""`       | delete field, config, and test               |      |
| 29 | `builds/repoprep.go:34`        | `resolvedSHA` return no production caller consumes           | drop the return                              |      |
| 30 | `srv/boundary_test.go:37`      | `go/parser` test policing an import convention               | delete; the import cycle already enforces it |      |
| 31 | `srv/boundary_test.go:19`      | `sharedPackages` hand-lists the repo layout                  | subsumed by 30                               |      |
| 32 | `github/app_test.go:47`        | asserts an unset env var is `""`                             | delete                                       |      |
| 33 | `builds/api.go:27-71`          | `buildResponse` restates `Build` field-for-field + copy loop | `json:` tags on `Build`; delete both         |      |
| 34 | `github/config.go:7`           | `ServerURLConfig` is the server's URL, read only by `auth`   | move to `srv/auth`                           |      |
| 35 | `srvtest/srvtest.go:62`        | shared scaffolding hardcodes one caller's route and id       | subsumed by 26                               |      |
| 36 | `auth/auth.go:80-119`          | `CurrentUser`/`CurrentSession` duplicate one join            | one private query, two projections           |      |
| 37 | `srv/srv.go:125-133`           | `connectOrNil` catch-log-continue; nil `*sqlx.DB` threaded   | propagate; hold "not connected" as a value   |      |
| 38 | `srv/srv.go:125-151`           | `connectDB` exists only to feed `connectOrNil`               | one function, after 37                       |      |
| 39 | `builds/runner.go:101,110`     | two catch-log-continue; a rationalization written in-source  | propagate `recordOutcome`                    |      |
| 40 | `install/state.go:49-105`      | every error flattened to a display string; `GetState` can't fail | `Entry` carries the error; return one     |      |
| 41 | three `migrations.go`          | three 8-line one-declaration files, comment triplicated      | fold each into its package's contract file   |      |
| 42 | `github/repo_names.go`         | one-function file, named for neither what it holds nor helps | move into `app.go`                           |      |
| 43 | `srv/ui.go`                    | one-type file for a 6-line controller mounted 4 lines away   | move into `srv.go`                           |      |
| 44 | `srv/pgerr/`                   | a package for two one-caller functions                       | inline each; delete the package              |      |
| 45 | `github/app.go:18`             | `var LoadApp = loadApp` — rebindable auth path for tests     | delete; tests supply a config context        |      |
| 46 | `builds/runner.go:23`          | `var publishBuild = runBuild`, citing 45 as precedent        | field on the runner struct                   |      |
| 47 | `builds/webhooks.go:139`       | one-caller helper, single-valued parameter                   | inline                                       |      |
| 48 | `builds/repoprep.go:109`       | `lockFile` one-caller helper                                 | inline into `syncMirror`                     |      |
| 49 | `install/state.go:16-26`       | `Status` stringly typed; a test returns an unrepresented ""  | `type Status string`, typed consts           |      |
| 50 | `builds/builds.go:42,55,79,93` | `out any` unused in 3 of 4; every callsite passes `nil`      | `Run(ctx) error`; `Claim` returns `*Build`   |      |
| 51 | `github/app.go:42`             | unnamed `\|\|` chain across five credential fields           | name them; report which is missing           |      |

### `releases/`, `engine/`, `git/`, `internal/`, `webui/` — 30 violations

`internal/` is **not** a grab-bag — three packages each named for a responsibility. Cleared.

| #  | Where                        | Violation                                                    | Fix                                        | Done |
|----|------------------------------|--------------------------------------------------------------|--------------------------------------------|------|
| 52 | `engine/clients.go:52,66`    | `_ = cached.Close()` — the banned form, twice                | propagate                                  |      |
| 53 | `git/git.go:19`+`context.go:106` | two git-exec owners in the self-declared "one boundary"   | one exec function                          |      |
| 54 | `engine/run.go:130`          | `client == nil` unreachable                                  | delete the branch                          |      |
| 55 | `engine/clients.go:17`       | `dial` seam, one assignment, no test override                | call `dialEngine`; delete the field        |      |
| 56 | `internal/timeouts:35`       | numeric branch kept for pre-`time.Duration` configs          | delete; parse error is correct             |      |
| 57 | `releases/releases.go:173`   | `"..HEAD"` stated three times, torn apart by `strings.Split` | pass the tag, not a range string           |      |
| 58 | `git/context.go:36`          | empty branch (detached HEAD) silently becomes `main`         | error                                      |      |
| 59 | `internal/buildlog:38`       | unsynchronized lazy init; `engine` fans out goroutines       | `sync.OnceValue` or lock                   |      |
| 60 | `engine/engine.go:97`        | unchecked `ctx.Value(...)` cast, panics on miss              | comma-ok + error                           |      |
| 61 | `releases/collection.go:101` | `chronoKey` panics, re-parses what `classOf` just parsed     | `classOf` returns the parsed key           |      |
| 62 | `engine/multiplexer.go:16`   | two-phase API for a one-shot op; nothing embeds it           | one `mapConcurrent` function               |      |
| 63 | `engine/multiplexer.go:24`   | `idx` unused in both closures                                | drop it                                    |      |
| 64 | `releases/collection.go:20`  | `Collection.cfg` never read; param exists to fill it         | delete both                                |      |
| 65 | `releases/releases.go:45`    | `Options.Name` never set or read                             | delete                                     |      |
| 66 | `releases/collection.go:113` | `Len`/`Names` no callers                                     | delete + the `iter` import                 |      |
| 67 | `releases/collection.go:156` | `PendingChanges` no callers                                  | delete                                     |      |
| 68 | `engine/engine.go:101`       | `LookupFromContext` exists to be `FromContext`'s counterpart | delete                                     |      |
| 69 | `internal/buildlog/events.go:48` | `GitInfo` no callers                                     | delete                                     |      |
| 70 | `releases/dateref:25,70`     | `New` dead; `Compare` kept alive only by its own test        | delete both + the test                     |      |
| 71 | `git/context.go:46`          | `CurrentBranch` exported, no external callers                | delete                                     |      |
| 72 | `git/context.go:72`          | `SetVersionTag` string return discarded by its only caller   | return only `error`                        |      |
| 73 | `git/context.go:87`          | `ListTags(pattern)` — one caller, always `"v*"`              | `ListVersionTags()`                        |      |
| 74 | `internal/buildlog/events.go:42` | `Git(cmd, ...)` one caller; renders as `git git=<args>`  | `Git(args ...string)`, fix the attr        |      |
| 75 | `releases/dateref:29,77`     | `Now(counter)` both callers pass 0; two unreachable guards   | `Now()`; delete the guards                 |      |
| 76 | `internal/buildinfo/`        | a package for two `Fprintln` wrappers, one consumer          | delete; write to stdout directly           |      |
| 77 | `buildinfo:11`,`buildlog:20` | `func out()` hiding `os.Stdout`/`os.Stderr`, no override     | inline                                     |      |
| 78 | `git/context.go` ×8          | eight doc comments restating their signatures                | delete all eight                           |      |
| 79 | `releases/datestamp.go:14`   | re-derives validity `dateref.IsValid` already provides       | delegate, like `Timestamp` does            |      |
| 80 | `releases/releases.go:131`   | `checkGitStatus` one caller, unused params, duplicate error  | inline; drop `releases.ErrDirtyWorkdir`    |      |
| 81 | `webui/webui_test.go:9`      | asserts an embed the compiler already refuses to build empty | delete                                     |      |

### `cmd/`, `gitops/` — 30 violations

**Two are live bugs, not style:** 82 and 84.

| #   | Where                        | Violation                                                   | Fix                                       | Done |
|-----|------------------------------|-------------------------------------------------------------|-------------------------------------------|------|
| 82  | `cmd/preview.go:102`         | **BUG** `--exec/-e` mutates the unit *after* the build       | apply before `engine.Build`, or delete    |      |
| 83  | `cmd/init/cmd.go:66`         | `err != nil \|\| fw == nil` swallows discovery failures      | propagate; `plan.go:113` does it right    |      |
| 84  | `cmd/vanity.go:55,59`        | **BUG** fatals on `ErrServerClosed` — nonzero exit on SIGTERM| exclude it; check `Close()`               |      |
| 85  | `cmd/preview.go:117`         | `tunnel.Stop` error discarded before `os.Exit(0)`            | check it                                  |      |
| 86  | `gitops/dsl/walk.go:131,139` | `Append` onto a non-list silently overwrites the value       | error when the cast fails                 |      |
| 87  | `gitops/render.go:189`       | error and empty-result fused; returns value + live error     | guard the error first                     |      |
| 88  | `cmd/preview.go:67`          | `preview == nil` unreachable                                 | delete                                    |      |
| 89  | `cmd/exec.go:97`,`vanity.go:33`,`export.go:46` | mutations inlined into commands            | own units, struct + `run(ctx)`            |      |
| 90  | `cmd/versions.go:25`         | `ok bool` re-threads what `info` carries; `(nil,true)` legal | drop `ok`                                 |      |
| 91  | `gitops/dsl/parse.go:289`    | four closures eta-wrapping functions of the same signature   | pass the functions                        |      |
| 92  | `gitops/dsl/parse.go:575`    | `execPathEdit`'s `fn` — one caller, always `Remove`          | drop the parameter                        |      |
| 93  | `gitops/dsl/parse.go:526`    | `reflect` recovering map identity the scope threw away       | scope carries doc provenance              |      |
| 94  | `gitops/dsl/path.go:25`      | `pathFromString` production code, only tests call it         | move to `_test.go`                        |      |
| 95  | `cmd/init/info.go:15`        | file named for `Info`, holds `resolveWD` with a dead branch  | inline; fold `Info`                       |      |
| 96  | `cmd/init/plan.go:113,173`   | `discover`/`specFileChanges` one-caller; stale doc comment   | inline both                               |      |
| 97  | `cmd/init/plan.go:162`       | package vars stranded between two functions                  | hoist                                     |      |
| 98  | `gitops/render.go:29`        | `DefaultRegistry` exported, read at one line in-package      | unexport                                  |      |
| 99  | `gitops/render.go:31-38`     | `appsPackage`/`platformExt` — fixed identifiers               | inline at the six sites                   |      |
| 100 | `gitops/tree.go:17`          | `merge` variadic, one callsite passing two                   | two named params                          |      |
| 101 | `cmd/init/cmd.go:90`         | the split `Apply`/`ApplyOverwrite` reassembled behind a bool  | two visibly different calls               |      |
| 102 | `cmd/init/cmd.go:97`         | re-derives `Plan.write`'s own skip rule                       | `Apply` returns what it wrote              |      |
| 103 | `cmd/exec.go:88`,`preview.go:53` | the unit-name loop duplicated                            | `Names()` on the owner                    |      |
| 104 | `cmd/preview.go:94`          | flag global used as working storage; unnamed source chain     | name both sources, merge locally          |      |
| 105 | `cmd/export.go:56`           | `else` after a branch that exits                              | flatten                                   |      |
| 106 | `gitops/dsl/*_test.go` ×6    | hand-rolled `reflect.DeepEqual`                               | `require.Equal`                           |      |
| 107 | `gitops/render_test.go:143`  | `equalStrings` hand-rolled sort-compare                       | `require.ElementsMatch`                   |      |
| 108 | `gitops/**_test.go`          | raw `t.Fatalf`/`t.Errorf`; `cmd/` already uses `require`      | convert                                   |      |
| 109 | `gitops/dsl/resolve_test.go` | named for a file that does not exist                          | merge into `lex_test.go`                  |      |
| 110 | `cmd/init` ×4                | "no app-vs-infra branch" narrated four times                  | keep once at most                         |      |
| 111 | `cmd/exec.go:52`,`preview.go:104` | comment restating a switch; a TODO on a dead line        | delete both                               |      |

## Finding 1 — blocked on the build budget

Two shapes were tried and **both blew the 1m budget** in `testbeds/*/platform.toml`, on all
three pnpm testbeds:

1. Delete both consts, `corepack enable pnpm` only, let corepack resolve `packageManager`
   when it first runs pnpm.
2. Read `.node-version` + `packageManager` host-side and bake them into the layer with
   `n install <ver>` + `corepack install -g pnpm@<ver>` — the shape the Go frameworks use
   for `go.mod`.

Reverted; the constants stand. What is **not** established is *why*. Three of the four runs
recompiled the pnpm base layer, so only the last was a fair warm measurement — it was red,
which rules out cache but does not name the slow step. Untested suspicion, recorded as
suspicion: moving `WithWorkdir(SrcDir)` ahead of the toolchain re-keys the `withBuildPkgs`
apk layer, so pnpm testbeds stop sharing it with the Go ones.

Next attempt should measure the step first (`dagger` step timings on one testbed), not
redesign on a guess. The timeout is not the variable — it is the budget the design has to
fit.

## Already landed this session

| Commit    | Change                                                          |
|-----------|-----------------------------------------------------------------|
| `a0da398` | Dropped the floating baseline preamble in infrabase.go           |
| `b409d35` | Killed the `-base` suffix — `infraComponents`/`Files`/`Dest`     |
| `cc565aa` | Folded the infra baseline into `platform_infra.go`               |
| `342881d` | `PlatformInfra`, `hasInfraName` inlined, three symbols unexported |
| `e2ae7b4` | Removed the reflection-based shape test                          |
| `c2080c4` | Moved each unit to the package owning it; `hasFile`; file renames |
| `197f2d5` | `ScaffoldVars`; folded `module.go` into `framework.go`            |
| `cf871d0` | `scaffold.Data` is a map — no per-framework fields                |
| `31572e8` | Inlined the fixed-identifier constants                           |
| `1adf240` | Added `go-coding-laws.md`; dropped `maps.Clone`                  |
| `c1573cb` | Two render trees, one `merge` point                              |
| `8bb38be` | Folded `declaredTags` into `computeCueTags`                      |
