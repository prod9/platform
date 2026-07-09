# Go build-speed regression — RESOLVED (`b2c292d`)

**Resolution:** the cost was the per-build Go-toolchain bootstrap. `withGoVersion` ran
`go install golang.org/dl/go<v>@latest` (network proxy resolve + shim compile) then
`go<v> download` (~150MB SDK pull) on **every** build; under the 7-bed parallel run those
network+compile steps contended and tipped go/basic over the 1m gate (marginally — 1 of 2
runs). Replaced with Go 1.21+ native `GOTOOLCHAIN=go<version>` + an explicit `go version`
probe placed before the go.mod copy, so the toolchain fetch is one 9s step keyed on the
version alone and Dagger-cached thereafter (warm = 0.0s CACHED). Full `./test.sh` now
UNCHANGED/green. Observed attribution: `withExec go version` 9.3s cold → 0.0s warm; the rest
of a warm go/basic build is ~0.6s. Kept for the diagnosis record; the symptom is closed.

---

Symptom (historical): `go/basic` and `go/workspace` **build** entries blew the 1m smoke gate
under the full parallel `./test.sh` run (CHANGED → `timed out after 1m0s`). The gate is a
deliberate performance assertion, not a flake tolerance — exceeding it *is* the regression to
fix, not to excuse. Do not raise the timeout.

## Established so far

- Standalone warm run is 3.05s — but that is a full Dagger layer-cache hit, not the real
  cost. Need **cold-cache** timing to see the true per-build work.
- Failure surfaces under concurrency (all testbeds building at once), so the cost is real
  per-build work that contends, not pure scheduler noise.

## Prime suspect — per-build Go toolchain fetch

`builder/go_shared.go` `withGoVersion`:

```
go install golang.org/dl/go<version>@latest   # network every build
go<version> download                            # ~150MB SDK pull unless cached
```

Mitigated by the `platform-go-sdk` CacheVolume, but: the `@latest` install still hits the
network each build, and any cold/missed SDK volume re-pulls the whole toolchain. go.mod
pins Go 1.25.5 while Wolfi's apk `go` is some other minor → GOTOOLCHAIN mismatch may force
the download path even when we think it's cached.

## To do in the deep pass

- Time a cold build (fresh Dagger cache) and attribute the seconds: apk, toolchain
  install/download, `mod download`, `go test`, `go build`.
- Decide whether the `golang.org/dl/go<v>@latest` bootstrap is worth its cost vs. pinning a
  Wolfi go package that already matches, or caching the dl shim.
- Re-check whether `go test -v ./...` (test-in-build gate) dominates on the workspace bed.
