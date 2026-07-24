" Vim syntax file
" Language: platform manifest-patch DSL (.platform files)
" Spec: prod9/platform docs/spec/manifest-patch-dsl.md

if exists("b:current_syntax")
  finish
endif

" Comments: # to end of line.
syn match platformDslComment "#.*$"

" Verbs at line start (longer alternatives first so `set` doesn't eat `set-if-absent`).
syn match platformDslVerb "^\s*\%(download\|extract\|emit\|focus\|reset\|set-if-absent\|set\|append-if-absent\|append\|remove-doc\|remove\)\>"

" Dotted paths: .spec.replicas, .[].kind, .a.b[0].type, .metadata."acme.io/x"
syn match platformDslPath "\.\%([A-Za-z0-9_-]\+\|\[[0-9]*\]\|\"[^\"]*\"\|\.\)*"

" Double-quoted strings with escapes and \(var) interpolation.
syn region platformDslString start=+"+ skip=+\\.+ end=+"+ contains=platformDslEscape,platformDslInterp
syn match platformDslEscape "\\[\\\"]" contained
syn match platformDslInterp "\\(\w\+)" contained

hi def link platformDslComment Comment
hi def link platformDslVerb    Statement
hi def link platformDslPath    Identifier
hi def link platformDslString  String
hi def link platformDslEscape  Special
hi def link platformDslInterp  Special

let b:current_syntax = "platform-dsl"
