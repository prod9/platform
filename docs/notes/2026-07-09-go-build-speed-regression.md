# Go build-speed regression — parked for deep analysis

Symptom: `go/basic` and `go/workspace` **build** entries blow the 1m smoke gate under the
full parallel `./test.sh` run (CHANGED → `timed out after 1m0s`). The gate is a deliberate
performance assertion, not a flake tolerance — exceeding it *is* the regression to fix, not
to excuse. Do not raise the timeout.

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
