# Authoring `.platform` files — the manifest-patch DSL in practice

How to write or patch a `.platform` component: the execution model in one screen and the
verbs you'll actually reach for. Exhaustive grammar (lexing, paths, escapes) lives in the
spec: [`../spec/manifest-patch-dsl.md`](../spec/manifest-patch-dsl.md).

## Editor setup

Vim/Neovim support ships in [`docs/guides/nvim/`](nvim/):
`syntax/platform-dsl.vim` (verbs, paths, strings, `\(var)` interpolation) and
`ftdetect/platform-dsl.vim` (associates `*.platform`). Copy or symlink both into your
runtimepath (`~/.config/nvim/`). Scaffolded `.platform` files also carry a
`# vim: ft=platform-dsl` modeline, so the syntax file alone is enough on machines
without the ftdetect.

## Mental model

A `.platform` file is a straight-line script over one working buffer — no branches, no
loops, no variables of its own. `download` fills the buffer with a foreign manifest
bundle; edits decode it lazily into a YAML document stream; `emit` writes the stream out.
Values come from `platform.toml` `[vars]`, through variable references or quoted
interpolation; an undefined variable is a hard error at render time.

The usual shape:

```
# vim: ft=platform-dsl
download "https://example.com/\(thing_version)/deploy.yaml"
focus .[].kind "NginxProxy"
set .spec.serverTokens "off"
reset
emit "thing.yaml"
```

`focus` narrows the edit scope (navigate or filter; chains narrow further — filter by
`kind`, then by `.metadata.name` to pick one of several Deployments), `reset` returns to
the whole stream, and edit paths are relative to the focused nodes. `set` auto-vivifies:
missing maps are created, `[N]` extends lists.

## The `set` family — existing implementation

All four run per focused node. Use quoted string values in the examples below; the
[DSL value grammar](../spec/manifest-patch-dsl.md) defines typed variable references and
interpolation, rather than this guide restating those rules.

| Verb                      | Writes when…                          | Keyed on          |
| ------------------------- | ------------------------------------- | ----------------- |
| `set PATH V`              | always                                | —                 |
| `set-if-absent PATH V`    | PATH has no value yet                 | **target** state  |
| `append PATH V`           | always (creates the list if missing)  | —                 |
| `append-if-absent PATH V` | V not already in the list             | **target** state  |

In the implementation (`dsl/parse.go`, the `exec` switch) each verb is a small
closure over the shared `execValueEdit` walk — `set-if-absent` is literally `set` behind
a `Get` check on the target path.

## No conditionals — by design

The DSL deliberately has no branches, truthiness, or optional-value verbs (a
`set-unless-empty` was proposed and rejected — it would smuggle empty-vs-nil semantics
into the language for one annotation). The patterns instead:

- **Environment-specific output** (a cloud LB annotation, a provider knob): don't ship it
  in a shared component at all — add the `set` directive in the repo where that
  environment lives. `.platform` files in an infra repo are the operator's committed
  files; editing them **is** the mechanism
  ([provider-neutral ADR](../decisions/2026-07-16-baseline-is-provider-neutral.md)).
- **A value the operator must supply**: reference the var and don't default it —
  interpolation of an undefined var is a hard render error, which is the "required"
  enforcement.
