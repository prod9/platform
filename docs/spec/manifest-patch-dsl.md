# Manifest Patch DSL

**Status:** accepted (rev. 2026-06-17) — verb set, grammar, lexer, and `\(var)`
interpolation settled with chakrit. **Slices D1–D2 landed** (in-buffer verbs, lexer,
path-walk, then `download`/`extract`/`emit` + interpolation) plus **D3a** (`Ops.Vars`
config passthrough) and **D3b-1** (bootstrap write-path: wd-validation + `[ops.vars]`
merge + plan/apply) and **D3b-2** (assembly layer: whole-file selection in `core/baseline`).
D3b split into D3b-1..4; D3b-3 (`ops render` routes `.cue`/`.platform` by extension) next.
**Decided in:**
[renderer ADR](../decisions/2026-06-16-renderer-cue-export-not-timoni.md),
[appliance ADR](../decisions/2026-06-17-opinionated-appliance-embedded-init.md). Build
plan: [roadmap](../notes/2026-06-16-platformv2-implementation-plan.md) Phase A′.

A line-oriented directive language for adapting third-party Kubernetes manifests we don't
own (cert-manager, NGINX Gateway Fabric, …) — fetch upstream, patch by name, write the
result to yaml files. It is **a yaml editor and nothing more**: it knows nothing about how
those files are later packaged or delivered (publish, OCI, Flux, git, `kubectl apply`) —
that is entirely downstream and out of scope. CUE handles manifests we author; this handles
foreign ones. Folded from infra-cli's `pipelines`
+ `pipelines/yamleditor` (~676 LOC incl. tests; the verbs already exist as Go pipeline ops
— only the directive parser and the field-select path form are new code). Its first
consumer is the **init DSL package** (the embedded cluster baseline), dogfooded against the
real `infra` repo (`apps/cert-manager.cue`, `k8s/nginx-gateway`, …).

## Why a closed vocabulary, not a script

With a general-purpose embedded language (Lua, Starlark, CEL, yq-expr) you can't tell what a
script does without mentally running it — tracking variables through branches and loops to
see what survives. A fixed set of verbs with no control flow removes that: each line does
one stated thing, every path names exactly one location, and reading the file top to bottom
*is* knowing its full effect. That's the whole point — directives stay auditable by reading,
the same reason Helm and TypeScript are banned here.

## Model

A directive file is a sequence of lines that edits a **working buffer** holding a
multi-document YAML stream, and writes results to **named output files**. `select` narrows
the active document scope; `reset` widens it back to the whole stream; subsequent edit verbs
apply to every document in scope, at the path each names. The back-end is the existing
`yamleditor` path-walk (`Get`/`Set` over `map[string]any` / `[]any`); the DSL is a thin
front-end over it.

**Two outputs, both explicit.**

- **Working buffer** — `download`/`extract` *replace* it; edit verbs (`select`, `set`,
  `remove`, …) mutate it.
- **Output files** — `emit FILENAME` writes the current working buffer to a named file,
  **replacing** it (truncate + write, not append). A script emits 0..N files; no `emit`
  means no output. Re-running a script reproduces the same file set deterministically —
  idempotent regeneration, like a codegen step. (Append-style accumulation was rejected: it
  grows files unboundedly across re-runs.)

`emit`'s filename is **relative**, resolved against an output directory the *runner*
provides — directive files name files, never absolute paths or repo layout, so they stay
portable. The command invoking the DSL decides where the tree lands. End-of-file is **not**
a sink: nothing is written unless `emit` says so.

A script's natural shape is therefore `download → patch → emit`, optionally repeated for
several components, each `emit` to its own filename. `download`/`extract` replacing the
working buffer is fine — the prior component was already captured to its file by `emit`
before the next `download` overwrites the work area.

**Branch-free by design:** no conditionals, no loops. Config-gating (include this patch only
in experimental mode) happens at *assembly* time, and at **whole-file** granularity — never
per-line, so a directive file is always read straight through. The assembly layer
(`core/baseline`) selects which files to apply from a filename convention, keyed off
`[ops.vars]`:

- `name.platform` — always applied.
- `name@variant.platform` — one variant of choice group `name`; applied when `vars[name] ==
  variant` (the lexically-first variant is the default when unset).
- `name+flag.platform` — an overlay; applied when `vars[flag] == "true"`.

(Directive files carry the `.platform` extension.)

`bootstrap` discovers these options from the embedded baseline and prompts the operator,
writing the chosen values into `[ops.vars]`; `--force` takes the defaults. `\(var)`
interpolation supplies values *within* a selected file; interpolation only, no expressions
(see **Variable interpolation**).

**Where `\(var)` values come from
([generic-ops-vars ADR](../decisions/2026-06-17-generic-ops-vars-single-config.md)):**
`platform.toml`'s `[ops.vars]` — a **generic open `map[string]string`**, stored verbatim by
the config processor (no per-software fields). The DSL owns its own variable vocabulary;
adding/removing a `\(var)` edits the directive file and `[ops.vars]`, never the Go DTO.
Values are strings — bools too (`experimental = "true"`), which the assembly layer
interprets to gate which directive *files* it applies (see **Branch-free by design**).
`[ops].image`/`tag` stay typed
(publish target, not a DSL var). `settings.toml` is eliminated.

**Defaults + re-bootstrap merge.** The embed ships the baseline's *default* `[ops.vars]`
(the versions/flags platform was tested against) alongside the directive files. First
`bootstrap` writes both into the infra repo. A later re-`bootstrap` after a platform upgrade
**overwrites the directive files** (platform's opinion, re-shipped) but **merges
`[ops.vars]`**: new keys are appended with their defaults, existing keys keep the operator's
value untouched. So a security bump (edit a var) survives the upgrade, and a newly
introduced baseline knob arrives pre-set instead of failing at render. Customization is via
vars — operator edits to a directive *file* are not preserved across re-bootstrap. The merge
is a surgical append of new `key = "value"` lines under `[ops.vars]`, not a decode/re-encode
(which would lose the operator's comments and ordering).

**Plan, then apply.** Bootstrap runs an analysis pass and prints the plan it would execute
— each file written or overwritten, each var appended vs. preserved — then confirms
interactively before touching the working tree. `--force` skips the prompt and applies the
plan unprompted (CI / non-interactive). The plan-and-confirm *is* the guard against a
surprise directive overwrite; there is no separate write-once refusal.

## Lexing & parsing

A directive file compiles in three passes — **lex → parse → execute** — so a syntax error
(a malformed path, an unknown directive) surfaces with its **line number** before any
download or disk write: `line 3: unknown directive "ste"`.

Each line is `verb arg…`. The lexer emits tokens (`.` `[` `]` `=`, identifiers, strings) and
records whitespace, so the parser knows where one argument ends and the next begins (a path's
segments are contiguous; arguments are whitespace-separated). `#` starts a comment to
end-of-line; blank lines are skipped. The parser builds a `Directive` (verb + typed args),
dispatching each argument by its leading token:

- **path** — leads with `.`; a **first-class selector** parsed into steps, never a string:
  `set .spec.replicas v`, `set .a.b[0].type v`. A dotted/slashed key is a quoted *segment*
  inside the otherwise-bare path: `set .metadata.annotations."acme.io/x" "y"`.
- **string** — `"…"`; double-quote only, escapes `\"` `\\`, and `\(var)` interpolation (see
  below). The only way to write a string-literal value.
- **variable reference** — a bare identifier in a value position (see **Value typing**).

```
set .spec.replicas replicas                # path, then a var reference
append .args "--feature-gates=Foo=true"    # path, then a string
set .metadata.annotations."acme.io/x" "y"  # quoted path segment for a dotted key
# full-line comment
```

## Variable interpolation

`\(NAME)` inside a double-quoted string is replaced by the value of var `NAME` — CUE's
interpolation syntax, so directive files and the `.cue` the renderer runs read the same way.

**Only inside strings.** Interpolation is a property of the string literal, so it is lexed
as part of one quoted token — a value that expands to text with spaces stays a single token,
never re-splitting a line's arity (the shell word-split footgun is structurally impossible).

```
set      .metadata.name "\(prefix)-controller"               # mid-string
download "https://github.com/.../\(version)/install.yaml"    # URL must be quoted to interpolate
```

**Value typing — bare is a reference, quoted is a string.** `[ops.vars]` is `map[string]any`,
so a var keeps its TOML type. The **value** position (the right side of `set`/`append`, the
match side of `select`) reads like CUE:

- A **bare token is a variable reference** — `set .spec.replicas replicas` writes the var's
  *native* type (int `3`, bool `true`, string). The name must be declared, else it is an error
  — there is **no** silent literal fallback (a bare token is never an accidental literal).
- A **quoted token is a string** — `set .spec.serverTokens "off"`, `select .kind "NginxProxy"`.
  `\(var)` interpolates **inside the quotes** (`"\(prefix)-controller"`), always producing a
  string. This is how a numeric-looking id stays a string: `firewall_id = "11222746"` (a string
  var) set via `"\(firewall_id)"`.
- A bare `\(x)` (interpolation sigil outside quotes) is a **syntax error** — quote it.

`set` never re-types (no YAML coercion): the type comes from the var, period. Structural
positions (the verb, paths, URLs, filenames) are *not* value positions — a bare `.kind` or
`download example.com` is literal text, never a variable lookup.

- **Name** — everything up to the closing `)` (e.g. `\(nginx-experimental)`).
- **Undefined var is a hard error**, not empty — a typo'd `\(verison)` must fail loudly, not
  silently blank a URL. (Lines are gated at assembly time, so any `\(x)` that reaches the
  parser is expected to be set.)
- **Literal `\(`** — escape the backslash: `\\(x)` renders the literal text `\(x)`. (Inside
  strings `\\` and `\"` already escape; this is the same rule.)
- **No expressions** — only `\(name)`. No defaults, no fallbacks, no arithmetic.
- **Guard:** a bare token containing `\(` is almost certainly a forgotten quote; the parser
  rejects it rather than silently emitting literal `\(x)`.

## Path grammar

Dotted keys, matched **literally at exactly the level named** (no aliasing, no search), with
two index forms:

- `.spec.replicas` — map keys.
- `.spec.containers[0]` — numeric list index.
- `.spec.containers[name=cert-manager-controller]` — field-select: the list element whose
  `name` equals the value. This is the load-bearing form — upstream reorders containers
  between versions, so index targeting is a latent bug.
- `.metadata.annotations."service.beta.kubernetes.io/linode-loadbalancer-firewall-id"` —
  **quoted key** (jq-style): a key containing `.`/`/` taken verbatim as one step. Needed for
  annotation/label keys.

Selection is by literal metadata path (`.metadata.name`, `.metadata.namespace`, `.kind`) —
there is no `.name`→`.metadata.name` shorthand. The structure is fixed; spell it out.

**`set` auto-vivifies the route.** A deep `set` creates missing intermediate maps and extends
list slots (`[0]` on an absent list creates it; the new element's type follows the next step),
so a structured value can be built from nothing via successive scalar `set`s — e.g. the NGF
`NginxProxy.spec.kubernetes.service.patches[0]` StrategicMerge entry. A missing `[field=val]`
element is still an error: there is no key to fabricate it by.

**Deferred extension — wildcard deep-select (`.*.name` / `.**`).** Not in D1. A wildcard
would let a path match at unknown depth, but that reintroduces "execute it mentally to know
what it hits" — the exact property the closed vocabulary exists to prevent — and we have no
real case (doc selection is fixed-structure metadata; list reordering is already handled by
`[field=val]`). If a need appears, pin the semantics (single-level `.*` vs recursive `.**`)
before adding it.

## Verbs

| Verb                      | Effect                                                          |
| ------------------------- | -------------------------------------------------------------- |
| `download URL`            | fetch into the buffer (replaces buffer)                        |
| `extract [PATH]`          | decompress/unarchive the buffer; PATH selects a member         |
| `select PATH V`           | narrow scope to docs where PATH equals V (cumulative AND)      |
| `reset`                   | widen scope back to the whole stream                           |
| `set PATH V`              | set scalar at PATH, creating intermediates                     |
| `set-if-absent PATH V`    | set only when PATH is unset (idempotent guard)                 |
| `append PATH V`           | append V to the list at PATH, creating it if absent            |
| `append-if-absent PATH V` | append only when V is not already in the list (idempotent)     |
| `remove PATH`             | remove the field or list element at PATH                       |
| `remove-doc`              | drop every document in scope from the stream                   |
| `emit "FILENAME"`         | write the working buffer to FILENAME (replace), relative to runner output dir |

Notes on individual verbs:

- **`extract`** detects the container format by **magic bytes on the buffer**, not the URL
  extension: gzip (`1f 8b`), zip (`50 4b`), tar (`ustar` at offset 257), composed in two
  layers (compression then archive) so `.tar.gz`, `.zip`, and a bare `.gz` all work. `PATH`
  selects a member inside an archive; it is **optional** — omit it for a single-stream
  decompression (bare `.gz`) where there is no inner member.
- **`select` is cumulative** — each `select` narrows the current scope by ANDing its
  constraint. Use **`reset`** to clear scope back to all docs before selecting a different
  document. Scope holds across edit verbs until the next `select`/`reset`.
- **Changing kind** is just `set .kind DaemonSet` — `.kind` is a path like any other; no
  dedicated verb.
- **`remove-doc`** is not expressible as `remove PATH`: a whole document is a top-level
  *stream element*, not a field at any path. Foreign bundles routinely ship docs to strip
  wholesale (a default `Secret`, a bundled `Namespace`, a `ServiceMonitor` you don't run).
- **`emit`** takes a quoted string filename (so it can interpolate, `emit "\(name).yaml"`).
  It replaces, never appends — emitting the same filename twice is last-wins. It is file I/O
  (lands in D2 with `download`); the DSL writes the file and is done. What reads the emitted
  tree is out of scope.

## Examples

cert-manager — append controller flags to a named container, version-robust, idempotent:

```
download "https://github.com/cert-manager/cert-manager/releases/download/\(version)/cert-manager.yaml"
select .kind "Deployment"
select .metadata.name "cert-manager"
append-if-absent .spec.template.spec.containers[name=cert-manager-controller].args "--enable-gateway-api"
append-if-absent .spec.template.spec.containers[name=cert-manager-controller].args "--feature-gates=ListenerSets=true"
emit "cert-manager.yaml"
```

NGF → DaemonSet — set a field, remove siblings:

```
select .kind "Deployment"
select .metadata.name "nginx-gateway"
set    .kind "DaemonSet"
remove .spec.replicas
remove .spec.strategy
emit   "nginx-gateway.yaml"
```

NGF serverTokens workaround, then argo doc-drop — `reset` between the two groups:

```
reset
select .kind "NginxProxy"
select .metadata.namespace "nginx-gateway"
set-if-absent .spec.serverTokens "off"

reset
select .kind "Secret"
select .metadata.name "argocd-secret"
remove-doc
```

archive source — extract a member, then patch:

```
download "https://example.com/some-operator-\(version).zip"
extract  "manifests/install.yaml"
select   .kind "Deployment"
set      .spec.replicas replicas
emit     "some-operator.yaml"
```

## Notes

- **Idempotency matters** — directives re-run on every upstream version bump, so
  `append-if-absent` / `set-if-absent` are first-class, not conveniences. `emit` replacing
  (not appending) its file is the same property at the output boundary: re-runs regenerate,
  not accumulate.
- **v2 tail** — infra-cli ended pipelines with `write` + `kubectl apply` + `git commit`. In
  v2 only the `write` survives, as `emit FILENAME`; apply/commit drop out because the DSL no
  longer delivers — it just writes files. The `download → extract → patch → emit` shape is
  the whole language; what consumes the emitted tree is downstream and unknown to the DSL.
- **Back-end reuse** — `yamleditor.Get`/`Set` already do path-walk with int-index list
  access and create-if-absent; the field-select form, the directive parser, and the lexer
  are the only new code.

## Build slices

The DSL lands across Phase A′ (see the
[roadmap](../notes/2026-06-16-platformv2-implementation-plan.md)):

- **D1 — DSL core (hermetic).** Path-walk + `[field=val]`, the in-buffer verbs (`select`,
  `reset`, `set`, `set-if-absent`, `append`, `append-if-absent`, `remove`, `remove-doc`),
  the lexer, and the directive parser. No network. Unit-tested on inline multi-doc
  fixtures. Born in `core/dsl`.
- **D2 — I/O verbs.** ✅ **Landed.** `download` (behind an injectable fetcher), `extract`
  (magic-byte gzip/zip/tar, two layers), `emit FILENAME` (truncate-write into a
  runner-provided output dir, no `..` escape), and `\(var)` interpolation (resolved in one
  left-to-right pass so `\\(` stays literal). Network verbs fixtured for tests, real fetch
  at runtime. Checksum guard deferred. How the emitted files reach a registry/cluster is a
  separate pipeline (`ops render`/`publish`), not part of the DSL.
- **D3a — `Ops.Vars` config passthrough.** ✅ **Landed.** `project.Ops` gained `Vars
  map[string]string` (`[ops.vars]`), stored verbatim — no defaults, no per-software fields.
  The source for `\(var)` values; the DSL already consumes it via `Options.Vars`.
- **D3b — baseline authoring + bootstrap-writes-DSL.** Split into four sub-slices, landing
  the hermetic mechanics before the content:
  - **D3b-1 — bootstrap write-path.** ✅ **Landed.** `bootstrapper.Analyze` computes a
    `Plan` (files written/overwritten, vars appended/preserved) without mutating; `Plan.Apply`
    writes it. wd-validation is a hard gate (target must exist, be a dir, live in a git repo);
    re-`bootstrap` merges `[ops.vars]` surgically (`mergeOpsVars`: append new keys, preserve
    operator values + comments/order, no decode/re-encode) instead of clobbering platform.toml.
    `bootstrap` prints the plan and confirms via fx prompt; `--force` applies unprompted. See
    **Defaults + re-bootstrap merge** and **Plan, then apply** above.
  - **D3b-2 — assembly layer.** ✅ **Landed** (`core/baseline`). Whole-file selection from a
    filename convention (`name.platform` / `name@variant.platform` / `name+flag.platform`) keyed off
    `[ops.vars]`; `ScanOptions` surfaces the operator-selectable knobs, `Select` resolves the
    file set (unknown choice value is a hard error). DSL stays branch-free.
  - **D3b-3 — `ops render` routes by extension + bootstrap option prompts.** Split in three:
    **3a (CUE file-map render+publish rework) landed**, **3b (`.platform` route) landed**, prompts
    → D3b-4. `ops render` walks the infra repo and dispatches per input type: `.cue` → file-map `cue export`,
    `.platform` → `baseline.Select` over `[ops.vars]` → `dsl.Apply` (download → patch →
    `emit`). Both write named files into a `k8s/<component>/` render-output tree (`core/baseline`
    owns the directive→dir mapping); `ops publish` packages it. Render-time, nothing rendered
    committed (model I). Bootstrap option prompts (from `baseline.ScanOptions`, written into
    `[ops.vars]`) land here too. Reworks Slice-1 render from the flat `-e objects` stream to
    the file-map contract. See the
    [render-routing ADR](../decisions/2026-06-18-render-routes-cue-and-platform-by-extension.md);
    supersedes the interim "separate run-DSL command (model II)" framing.
  - **D3b-4 — baseline authoring + migration.** The embedded baseline (Flux/cert-manager/NGF/
    engine) as `.platform` directive files + default `[ops.vars]`, `go:embed`'d, written by
    bootstrap; fold `settings.toml` into `platform.toml` and delete it. Follows D3b-3 so the
    directive files verify end-to-end.
