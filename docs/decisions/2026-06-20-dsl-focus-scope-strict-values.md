# Patch DSL: focus/reset scope, strict values, a real lexer/parser

- **Status:** accepted
- **Date:** 2026-06-20
- **From:** D3b-4 DSL design pass with chakrit. Revises the interim grammar settled in the
  [manifest-patch-DSL spec](../spec/manifest-patch-dsl.md) (rev. 2026-06-17) and parts of D1–D2.

## Context

Authoring the real baseline directives (NGF firewall annotation, cert-manager controller args)
stressed the D1 grammar and surfaced three problems, each fixed by reversing an earlier shortcut.

## Decisions

1. **`focus` / `reset` scope, not `select` / `[field=value]`.** Find-a-thing-by-identity lives in
   one verb. `focus PATH [VALUE]` narrows the scope into the document tree — navigate
   (`.[]`/`.key`/`[N]`) or filter (a trailing `field == VALUE`). It chains (each narrows within
   the previous), edits then run with paths **relative** to the focused nodes, and `reset` returns
   to the whole stream. The shape is `focus → edit → reset`, repeated per group (rarely >2 groups,
   so a flat single scope — not a push/pop stack — was deemed enough). The old `select PATH VALUE`
   (doc-only filter) + `[field=value]` path step are both gone.

2. **No `[field=value]` selector at all.** Brackets hold only `[N]` (index) or `[]` (iterate).
   Killing the bracketed field=value removes the special bracket lexing (the raw "field until `=`,
   value until `]`" scan, and the `[k=x=y]` edge case) and drops `=` from the grammar. Matching a
   list element by field is now `focus .…containers[].name "ctl"` → edit relative. Filter semantics
   split a focus path at its **last `[]`**: segments before it navigate to candidates, segments
   after are the predicate tested on each (so `focus .metadata.name "a"` keeps the *doc*, not the
   `.metadata` map).

3. **Strict value typing — bare is a variable reference, quoted is a string.** `[ops.vars]` is
   `map[string]any` (TOML keeps int/bool/string). In a value position a bare token is a var
   reference yielding the native type (undefined is an error — no silent literal fallback); a
   quoted `"…"` is a string (the only string-literal form; `\(var)` interpolates inside it, always
   stringifying). `set` never re-types via YAML. There is no bare `\(x)` — interpolation lives only
   inside quotes. (Rejected: lenient "var-if-declared-else-literal" — ambiguous; and `"\(x)"` as a
   force-string — type comes from the var, so a string var like `firewall_id = "11222746"` already
   yields a string.)

4. **A real front-end: lex → parse → execute.** The path is a first-class grammar production
   parsed into steps from tokens — not a quoted string character-scanned at runtime. `Parse`
   compiles the whole file into `[]Directive` (verb + typed args + line number) up front, so syntax
   errors and `line 3: unknown directive "ste"` surface before any download or disk write.

## Consequences

- `core/dsl` lexer/parser/path/walk and the engine scope were rewritten (`select`→`focus`, scope
  is the focused node set, `Select` path step deleted). Edit paths are Key/Index only; `[]` is
  focus-only. The NGF directive is `focus .[].kind "NginxProxy"` + relative sets; dogfooded against
  live upstream, output matches the infra repo's committed `k8s/nginx-gateway/nginx-gateway.yaml`.
- The DSL spec is the authoritative grammar; this records the rationale and the reversals.
- **Process note for this work:** language/API *design and naming* get settled with chakrit before
  implementation; jumping into the editor mid-decision (e.g. starting the lexer while a verb-name
  pair was still being chosen) cost several redo loops here.
