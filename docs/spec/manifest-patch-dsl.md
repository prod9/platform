# Manifest Patch DSL

**Status:** living spec. Verb set, grammar, lexer, path-walk, `download`/`extract`/`emit`,
and `\(var)` interpolation are landed and in use; the embedded baseline authors its foreign
components (cert-manager, NGF, …) as `.platform` files rendered by `render`. The baseline is
a flat list installed **unconditionally** (no picker); the `Infra` framework owns which
`.platform` files ship — see the
[flat-baseline ADR](../decisions/2026-06-22-flat-baseline-install-time-selection.md) and
[baseline-dissolves-into-infra-framework](../decisions/2026-07-11-baseline-dissolves-into-infra-framework.md).
**Decided in:**
[renderer ADR](../decisions/2026-06-16-renderer-cue-export-not-timoni.md),
[appliance ADR](../decisions/2026-06-17-opinionated-appliance-embedded-init.md).

A line-oriented directive language for adapting third-party Kubernetes manifests we don't
own (cert-manager, NGINX Gateway Fabric, …) — fetch upstream, patch by name, write the
result to yaml files. It is **a yaml editor and nothing more**: it knows nothing about how
those files are later packaged or delivered (publish, OCI, Flux, git, `kubectl apply`) —
that is entirely downstream and out of scope. CUE handles manifests we author; this handles
foreign ones. Folded from infra-cli's `pipelines`
+ `pipelines/yamleditor` (~676 LOC incl. tests; the verbs already exist as Go pipeline ops
— only the directive parser and the field-select path form are new code). Its first
consumer is the **embedded cluster baseline** (a flat list of `.platform` + `.cue` files
the `Infra` framework installs unconditionally), dogfooded against the
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
multi-document YAML stream, and writes results to **named output files**. `focus` narrows the
scope into the document tree (chaining to descend); `reset` returns it to the whole stream;
subsequent edit verbs apply to every focused node, at the path each names relative to it. The back-end is the existing
`yamleditor` path-walk (`Get`/`Set` over `map[string]any` / `[]any`); the DSL is a thin
front-end over it.

**Two outputs, both explicit.**

- **Working buffer** — `download`/`extract` *replace* it; edit verbs (`focus`, `set`,
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

**Branch-free by design:** no conditionals, no loops. Installation is at **whole-file**
granularity — never per-line, so a directive file is always read straight through. The
embedded baseline is one **flat list** of component files (`.platform` directives + `.cue`
apps, clean names) that the `Infra` framework installs **unconditionally** at `init` — no
picker, no `Defaults`/`Mandatory` split; an operator prunes what they don't want by editing
the committed repo afterward. `render` then applies whatever is present, routing by
extension. There is **no** filename marker grammar (`@variant`, `+flag`), **no** render-time
gating on `[vars]`, and **no** assembly `Select` step — see the
[flat-baseline ADR](../decisions/2026-06-22-flat-baseline-install-time-selection.md) and
[baseline-dissolves-into-infra-framework](../decisions/2026-07-11-baseline-dissolves-into-infra-framework.md).

(Directive files carry the `.platform` extension.)

`\(var)` interpolation supplies values *within* an applied file — version pins from
`[vars]`, interpolation only, no expressions (see **Variable interpolation**). Selection
is **not** a var.

**Where `\(var)` values come from
([generic-ops-vars ADR](../decisions/2026-06-17-generic-ops-vars-single-config.md)):**
`platform.toml`'s `[vars]` — a **generic open `map[string]any`**, stored verbatim by
the config processor (no per-software fields), each value keeping its TOML type
(string/int/bool). The DSL owns its own variable vocabulary; adding/removing a `\(var)` edits
the directive file and `[vars]`, never the Go DTO. A bare token in a value position
resolves to the var's **native** type (see **Value typing**); `[vars]` carries version
pins, not selection toggles. The image ref and tag are not DSL vars — the image is inferred
per-module and the tag derives from the release strategy, never part of `[vars]`.
`settings.toml` is eliminated.

**Defaults + re-init merge.** The embed ships the baseline's *default* `[vars]`
(the version pins platform was tested against) alongside the directive files. First
`init` writes both into the infra repo. A later re-`init` after a platform upgrade
**overwrites the directive files** (platform's opinion, re-shipped) but **merges
`[vars]`**: new keys are appended with their defaults, existing keys keep the operator's
value untouched. So a security bump (edit a var) survives the upgrade, and a newly
introduced baseline knob arrives pre-set instead of failing at render. Customization is via
vars — operator edits to a directive *file* are not preserved across re-init. The merge
is a surgical append of new `key = "value"` lines under `[vars]`, not a decode/re-encode
(which would lose the operator's comments and ordering).

**Plan, then apply.** `init` runs an analysis pass and prints the plan it would execute
— each file written or overwritten, each var appended vs. preserved — then confirms
interactively before touching the working tree. `--force` skips the prompt and applies the
plan unprompted (CI / non-interactive). The plan-and-confirm *is* the guard against a
surprise directive overwrite; there is no separate write-once refusal.

## Lexing & parsing

A directive file compiles in three passes — **lex → parse → execute** — so a syntax error
(a malformed path, an unknown directive) surfaces with its **line number** before any
download or disk write: `line 3: unknown directive "ste"`.

Each line is `verb arg…`. The lexer emits tokens (`.` `[` `]`, identifiers, strings) and
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

**Value typing — bare is a reference, quoted is a string.** `[vars]` is `map[string]any`,
so a var keeps its TOML type. The **value** position (the right side of `set`/`append`, the
match side of `focus`) reads like CUE:

- A **bare token is a variable reference** — `set .spec.replicas replicas` writes the var's
  *native* type (int `3`, bool `true`, string). The name must be declared, else it is an error
  — there is **no** silent literal fallback (a bare token is never an accidental literal).
- A **quoted token is a string** — `set .spec.serverTokens "off"`, `focus .[].kind "NginxProxy"`.
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

Dotted keys, matched **literally at exactly the level named** (no aliasing, no search), plus
brackets for lists:

- `.spec.replicas` — map keys.
- `.spec.containers[0]` — numeric list index (glued to the key; or leading `.[0]`).
- `.spec.containers[]` — **iterate** every element. Focus-only: an edit addresses one node, so
  `[]` is rejected in a `set`/`append`/`remove` path.
- `.metadata.annotations."service.beta.kubernetes.io/linode-loadbalancer-firewall-id"` —
  **quoted key** (jq-style): a key containing `.`/`/` taken verbatim as one step.

**There is no `[field=value]` selector.** Matching a list element by a field is `focus`'s job
(see Scope): `focus .spec.containers[].name "cert-manager-controller"` narrows the scope to the
matching container, then edits run relative to it. Find-by-identity lives in one place (`focus`),
and the path grammar stays a tiny literal walk — no special bracket lexing, no `=`.

**`set` auto-vivifies the route** — missing maps are created and `[N]` extends the list — so a
nested value can be built from nothing via successive scalar `set`s, e.g. the NGF
`NginxProxy.spec.kubernetes.service.patches[0]` StrategicMerge entry.

## Verbs

| Verb                      | Effect                                                          |
| ------------------------- | -------------------------------------------------------------- |
| `download URL`            | fetch into the buffer (replaces buffer)                        |
| `extract [PATH]`          | decompress/unarchive the buffer; PATH selects a member         |
| `focus PATH`              | narrow scope by navigating (`.[]`/`.key`/`[N]`) into the tree  |
| `focus PATH V`            | narrow scope to nodes whose PATH-tail equals V (a filter)      |
| `reset`                   | scope back to the whole document stream                        |
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
- **`focus` chains** — each `focus` narrows *within* the current scope: navigate deeper
  (`focus .spec.template.spec.containers[].name "ctl"`) or filter (`focus .[].kind "X"`).
  Edits then apply to the focused nodes with paths **relative** to them. **`reset`** returns to
  the whole stream; the usual shape is focus→edit→`reset`, repeated per group. `.[]` iterates
  the current collection (the stream, or a list field); a leading `.[]` turns the document
  stream into its individual documents.
- **Changing kind** is just `set .kind "DaemonSet"` — `.kind` is a path like any other; no
  dedicated verb.
- **`remove-doc`** is not expressible as `remove PATH`: a whole document is a top-level
  *stream element*, not a field at any path. Foreign bundles routinely ship docs to strip
  wholesale (a default `Secret`, a bundled `Namespace`, a `ServiceMonitor` you don't run).
- **`emit`** takes a quoted string filename (so it can interpolate, `emit "\(name).yaml"`).
  It replaces, never appends — emitting the same filename twice is last-wins. It is file I/O
  (lands in D2 with `download`); the DSL writes the file and is done. What reads the emitted
  tree is out of scope.

## Examples

cert-manager — focus down to a named container, then append controller flags idempotently:

```
download "https://github.com/cert-manager/cert-manager/releases/download/\(version)/cert-manager.yaml"
focus .[].kind "Deployment"
focus .metadata.name "cert-manager"
focus .spec.template.spec.containers[].name "cert-manager-controller"
append-if-absent .args "--enable-gateway-api"
append-if-absent .args "--feature-gates=ListenerSets=true"
emit "cert-manager.yaml"
```

NGF → DaemonSet — focus a doc, set a field, remove siblings:

```
focus  .[].kind "Deployment"
focus  .metadata.name "nginx-gateway"
set    .kind "DaemonSet"
remove .spec.replicas
remove .spec.strategy
emit   "nginx-gateway.yaml"
```

NGF serverTokens workaround, then argo doc-drop — `reset` between the two groups:

```
reset
focus .[].kind "NginxProxy"
set-if-absent .spec.serverTokens "off"

reset
focus .[].kind "Secret"
focus .metadata.name "argocd-secret"
remove-doc
```

archive source — extract a member, then patch:

```
download "https://example.com/some-operator-\(version).zip"
extract  "manifests/install.yaml"
focus    .[].kind "Deployment"
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

## Implementation

The DSL lives in `dsl/` (path-walk, the in-buffer verbs, lexer, directive parser, and the
I/O verbs `download`/`extract`/`emit` with `\(var)` interpolation). `[vars]` reaches it
verbatim as `Options.Vars` — `project.Ops.Vars` is a `map[string]any` stored with no
defaults or per-software fields (see the
[generic-ops-vars ADR](../decisions/2026-06-17-generic-ops-vars-single-config.md)). The
checksum guard is deferred.

Its consumers own the surrounding flow, specced elsewhere:

- **`init`** writes the embedded baseline's `.platform` + `.cue` files and default
  `[vars]` into the infra repo, plan-then-apply with the surgical re-init var merge —
  see [`scaffolding.md`](scaffolding.md).
- **`render`** walks the infra repo's `apps/` and routes by extension: `.cue` → file-map
  export via the linked CUE evaluator (no `cue` binary), `.platform` → `dsl.Apply` over every
  present directive file. Both land named files in a `k8s/<component>/` tree that the infra
  framework packs into the published image — see [`architecture.md`](architecture.md) and the
  [render-routing ADR](../decisions/2026-06-18-render-routes-cue-and-platform-by-extension.md).
