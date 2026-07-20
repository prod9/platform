# Go coding laws

Running list. Each law was hammered in after code in this repo broke it — the instance is
named so a later scan can recognize the same shape. Append; don't prune.

## Shape

1. **One uniform shape per framework.** A framework's Go type and its source file both
   mirror its `Name()`: `go/basic` is `GoBasic` in `go_basic.go`, `platform/infra` is
   `PlatformInfra` in `platform_infra.go`. *Broke it:* `type Infra` in `platform_infra.go`
   alongside an orphan `infra.go`.
2. **A file is named for what it holds, never for its consumer.** *Broke it:*
   `infrabase.go` welded a file-set onto the builder that reads it; `scaffold.go` held only
   `defaultModule`, which builds a `conf.Module` and scaffolds nothing.
3. **No file for one random function.** A lone shared helper joins the package's contract
   file. *Broke it:* `detect.go` (one `os.Stat` wrapper), `module.go` (one function).
4. **Interface methods sit together.** Don't strand one below a pile of package-level data.
   *Broke it:* `Build` at the bottom of `platform_infra.go`, eight declarations away from
   the rest.

## Ownership

5. **Data belongs to the package that owns the concern, not the one that reads it.**
   *Broke it:* `infraDest` decoded skel's own `apps-`/`defaults-` filename convention from
   outside skel.
6. **A generic mechanism grows no per-consumer fields.** *Broke it:* `scaffold.Data` was a
   struct with a field per template hole, so infra's cue.mod pins landed in the generic
   package — now a `map[string]any` keyed by the hole name.
7. **Never restate a fact that already exists somewhere.** *Broke it:* `infraComponents`
   hand-listed the nine filenames `//go:embed apps-* defaults-*` already selects;
   `PNPMVersion`/`NodeVersion` restate what the built repo's own `package.json` declares.
8. **A one-caller helper is inlined.** *Broke it:* `hasInfraName`, called only by
   `Discover`.

## Naming

9. **A bool-returning function reads as a predicate.** `hasFile`, not `detectFile`.
10. **Sibling methods read as siblings.** `Scaffold` and `ScaffoldVars`, not `Scaffold` and
    `RequiredScaffoldInputs`.
11. **Unexport what nothing outside reads**, especially generic names on specific data.
    *Broke it:* `DefaultVars`/`DefsModule`/`DefsVersion`, package-internal the whole time.
12. **Check a proposed name against concepts already live** in the domain before adopting
    it.

## Composition

13. **Two producers means two chains and one merge point** — never grow one result by
    handing it to the next stage, and never fuse with `maps.Copy` at the callsite. Each
    route builds and returns its own value; a named `merge` combines them.
    *Broke it:* `Render` built the CUE tree, then `maps.Copy`'d the directives tree over it.
14. **Chained checks use `if … ; err != nil { } else if … ; err != nil { } else { }`.** The
    values stay scoped to the chain that produced them.

    ```go
    if cueTree, err := renderCueTree(srcDir, vars); err != nil {
        return nil, err
    } else if platformTree, err := renderPlatformTree(srcDir, vars, opts.Fetch); err != nil {
        return nil, err
    } else {
        return merge(cueTree, platformTree), nil
    }
    ```

## Constants and errors

15. **No `const` for a fixed identifier that cannot change.** Inline it; a name far from
    the one line that reads it costs a jump and buys nothing. *Broke it:*
    `cueModPrefixInput`, `daggerModule`, `varsHeader`. A const naming a *chosen policy*
    (`pollInterval`, `maxWebhookBody`) or a layout (`dateFormat`) stays.
16. **Never return an error no caller consumes.** Seven `detected, _ := detectFile(...)`
    callsites meant the error return was dead; the fix is dropping the return, not
    swallowing it seven times.

## Enforcement

17. **The type system or nothing — never police a convention with a test.** A
    reflection-based shape test is the tell that the design is wrong, and half of what it
    asserted (which file a declaration sits in) is not a fact Go can hold. Conventions are
    followed and written down; `var _ Framework = X{}` is the assertion the compiler can
    actually make.

## Conduct

18. **Don't defend code by its provenance.** Every commit here is agent-authored; naming an
    earlier one explains nothing and answers nothing.
19. **Don't rationalize a design under review.** When the shape is called bad, re-derive it —
    don't narrate why the current one was reasonable.
