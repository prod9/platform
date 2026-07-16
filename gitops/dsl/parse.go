package dsl

import (
	"fmt"
	"reflect"
	"strings"
)

// Options carries the run context for a directive file: the initial decoded
// buffer (download replaces it), the \(var) interpolation table, the output
// directory emit writes under, and an optional fetcher for download (nil uses a
// plain HTTP GET; tests inject fixtures).
type Options struct {
	Docs   []Doc
	Vars   Vars
	OutDir string
	Fetch  func(url string) ([]byte, error)
}

// argKind tags an argument by its surface syntax.
type argKind int

const (
	argPath argKind = iota // .a.b[0]  — a selector
	argStr                 // "quoted" — a string literal/interpolation
	argVar                 // bare ident — a variable reference
)

// Arg is one parsed directive argument, resolved against vars at execution.
type Arg struct {
	kind argKind
	path []pathSeg // argPath
	str  []strPart // argStr
	name string    // argVar
}

// segKind tags a path segment.
type segKind int

const (
	segKey   segKind = iota // .name
	segIndex                // [0]
	segIter                 // []  — iterate a list (focus only)
)

// pathSeg is one parsed path segment. A key carries string parts so a quoted key
// may interpolate (\(var)); index is a literal; iter carries nothing.
type pathSeg struct {
	kind  segKind
	key   []strPart // segKey
	index int       // segIndex
}

// Directive is one parsed, executable line: a verb, its arguments, and the
// source line number for diagnostics.
type Directive struct {
	Verb string
	Args []Arg
	Line int
}

// Parse compiles a directive file into executable directives. Syntax errors
// surface here — before any download or disk write — with their line number.
func Parse(src string) ([]Directive, error) {
	var prog []Directive
	for n, line := range strings.Split(src, "\n") {
		toks, err := lexLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", n+1, err)
		}
		if len(toks) == 0 {
			continue
		}

		d, err := parseLine(toks)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", n+1, err)
		}
		d.Line = n + 1
		prog = append(prog, *d)
	}
	return prog, nil
}

// parseLine turns a line's tokens into a Directive: a verb followed by
// arguments, each dispatched by its leading token.
func parseLine(toks []token) (*Directive, error) {
	if toks[0].kind != tIdent {
		return nil, fmt.Errorf("expected a directive name, got %s", toks[0].describe())
	}

	d := &Directive{Verb: toks[0].text}
	for i := 1; i < len(toks); {
		arg, next, err := parseArg(toks, i)
		if err != nil {
			return nil, err
		}
		d.Args = append(d.Args, arg)
		i = next
	}
	return d, nil
}

// parseArg parses one argument starting at toks[i], returning the index past it.
func parseArg(toks []token, i int) (Arg, int, error) {
	switch toks[i].kind {
	case tStr:
		return Arg{kind: argStr, str: toks[i].parts}, i + 1, nil
	case tIdent:
		return Arg{kind: argVar, name: toks[i].text}, i + 1, nil
	case tDot:
		return parsePath(toks, i)
	default:
		return Arg{}, 0, fmt.Errorf("unexpected %s", toks[i].describe())
	}
}

// parsePath parses a selector: a contiguous run of .key, [N], or [] segments
// (a whitespace-led token ends the path, starting the next argument). A bracket
// follows either a '.' (.[]/.[ 0]) or a key (patches[0]) — both spellings parse.
func parsePath(toks []token, i int) (Arg, int, error) {
	var steps []pathSeg
	for first := true; i < len(toks); first = false {
		if !first && toks[i].spaced {
			break
		}

		switch toks[i].kind {
		case tDot:
			i++
			if i >= len(toks) {
				return Arg{}, 0, fmt.Errorf("expected a key or '[' after '.'")
			}
			if toks[i].kind == tLBrack {
				continue // a bracket segment, handled below
			}
			if toks[i].kind != tIdent && toks[i].kind != tStr {
				return Arg{}, 0, fmt.Errorf("expected a key after '.'")
			}
			steps = append(steps, keySeg(toks[i]))
			i++

		case tLBrack:
			step, next, err := parseBracket(toks, i)
			if err != nil {
				return Arg{}, 0, err
			}
			steps = append(steps, step)
			i = next

		default:
			return Arg{}, 0, fmt.Errorf("unexpected %s in path", toks[i].describe())
		}
	}

	return Arg{kind: argPath, path: steps}, i, nil
}

func keySeg(t token) pathSeg {
	if t.kind == tStr {
		return pathSeg{kind: segKey, key: t.parts}
	}
	return pathSeg{kind: segKey, key: []strPart{{text: t.text}}}
}

// parseBracket parses a [N] index or [] iterate segment.
func parseBracket(toks []token, i int) (pathSeg, int, error) {
	i++ // consume '['
	if i >= len(toks) {
		return pathSeg{}, 0, fmt.Errorf("unclosed '['")
	}

	var step pathSeg
	if toks[i].kind == tIdent {
		n, err := parseIndex(toks[i].text)
		if err != nil {
			return pathSeg{}, 0, err
		}
		step = pathSeg{kind: segIndex, index: n}
		i++
	} else {
		step = pathSeg{kind: segIter}
	}

	if i >= len(toks) || toks[i].kind != tRBrack {
		return pathSeg{}, 0, fmt.Errorf("expected ']'")
	}
	return step, i + 1, nil
}

func parseIndex(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid list index %q", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// resolvePath materializes a parsed selector as an edit Path (a single node):
// keys interpolate, index is literal. An iterate ([]) belongs to focus, not an
// edit, so it is rejected here.
func resolvePath(steps []pathSeg, vars Vars) (Path, error) {
	path := make(Path, 0, len(steps))
	for _, s := range steps {
		switch s.kind {
		case segKey:
			name, err := resolveStr(s.key, vars)
			if err != nil {
				return nil, err
			}
			path = append(path, Key{Name: name})
		case segIndex:
			path = append(path, Index{N: s.index})
		case segIter:
			return nil, fmt.Errorf("'[]' iterate is not allowed in an edit path; focus into it first")
		}
	}
	return path, nil
}

// interpreter carries the directive interpreter's state. The buffer is two-state:
// raw holds undecoded bytes after download/extract; docs holds the decoded
// stream once an edit or emit forces a decode. decoded says which is live.
type interpreter struct {
	raw     []byte
	docs    []Doc
	decoded bool
	scope   []any // the focused nodes; reset is the whole doc stream

	vars   Vars
	outDir string
	fetch  func(url string) ([]byte, error)
}

// Apply parses directives and runs them against the buffer described by opts,
// returning the resulting decoded stream. Scope starts at the whole stream;
// select narrows it, reset widens it back. download/extract replace the buffer
// with raw bytes, decoded lazily the next time an edit or emit needs documents.
func Apply(directives string, opts Options) ([]Doc, error) {
	program, err := Parse(directives)
	if err != nil {
		return nil, err
	}

	e := &interpreter{
		docs:    opts.Docs,
		decoded: true,
		vars:    opts.Vars,
		outDir:  opts.OutDir,
		fetch:   opts.Fetch,
	}
	if e.fetch == nil {
		e.fetch = httpGet
	}
	e.resetScope()

	for _, d := range program {
		if err := e.exec(d); err != nil {
			return nil, fmt.Errorf("line %d: %w", d.Line, err)
		}
	}

	if err := e.ensureDecoded(); err != nil {
		return nil, err
	}
	return e.docs, nil
}

func (e *interpreter) exec(d Directive) error {
	switch d.Verb {
	case "download":
		return e.execDownload(d.Args)
	case "extract":
		return e.execExtract(d.Args)
	case "emit":
		return e.execEmit(d.Args)
	case "focus":
		return e.execFocus(d.Args)
	case "reset":
		return e.execReset(d.Args)
	case "set":
		return e.execValueEdit(d.Args, func(doc Doc, p Path, v any) error {
			return Set(doc, p, v)
		})
	case "set-if-absent":
		return e.execValueEdit(d.Args, func(doc Doc, p Path, v any) error {
			if _, ok := Get(doc, p); ok {
				return nil
			}
			return Set(doc, p, v)
		})
	case "append":
		return e.execValueEdit(d.Args, func(doc Doc, p Path, v any) error {
			return Append(doc, p, v)
		})
	case "append-if-absent":
		return e.execValueEdit(d.Args, func(doc Doc, p Path, v any) error {
			return AppendIfAbsent(doc, p, v)
		})
	case "remove":
		return e.execPathEdit(d.Args, func(doc Doc, p Path) error {
			return Remove(doc, p)
		})
	case "remove-doc":
		return e.execRemoveDoc(d.Args)
	default:
		return fmt.Errorf("unknown directive %q", d.Verb)
	}
}

// download URL — fetch into the buffer, replacing it with raw bytes.
func (e *interpreter) execDownload(args []Arg) error {
	if len(args) != 1 {
		return fmt.Errorf("download: want URL, got %d args", len(args))
	}
	url, err := e.argString(args[0])
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	data, err := e.fetch(url)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	e.setRaw(data)
	return nil
}

// extract [PATH] — decompress/unarchive the raw buffer in place. PATH selects an
// archive member; omit it for a bare compressed stream.
func (e *interpreter) execExtract(args []Arg) error {
	if len(args) > 1 {
		return fmt.Errorf("extract: want [PATH], got %d args", len(args))
	}
	if e.decoded {
		return fmt.Errorf("extract: nothing to extract (no prior download)")
	}

	member := ""
	if len(args) == 1 {
		m, err := e.argString(args[0])
		if err != nil {
			return fmt.Errorf("extract: %w", err)
		}
		member = m
	}
	data, err := extract(e.raw, member)
	if err != nil {
		return err
	}
	e.setRaw(data)
	return nil
}

// emit FILENAME — write the working buffer to a runner-relative file.
func (e *interpreter) execEmit(args []Arg) error {
	if len(args) != 1 {
		return fmt.Errorf("emit: want FILENAME, got %d args", len(args))
	}
	name, err := e.argString(args[0])
	if err != nil {
		return fmt.Errorf("emit: %w", err)
	}
	if err := e.ensureDecoded(); err != nil {
		return err
	}
	return emit(e.outDir, name, e.docs)
}

// focus PATH [VALUE] — narrow the scope into the document tree. With no VALUE it
// navigates: the new scope is every node the path reaches (.[] iterates a list,
// .key/[N] descend). With a VALUE it filters: the path's trailing .key is a
// predicate, and the scope becomes the nodes whose that field equals VALUE.
// focus chains — each one narrows within the previous; reset returns to the top.
func (e *interpreter) execFocus(args []Arg) error {
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("focus: want PATH or PATH VALUE, got %d args", len(args))
	}
	if args[0].kind != argPath {
		return fmt.Errorf("focus: first argument must be a path (.a.b)")
	}
	if err := e.ensureDecoded(); err != nil {
		return err
	}
	segs := args[0].path

	if len(args) == 1 {
		scope, err := e.walkFocus(e.scope, segs)
		if err != nil {
			return fmt.Errorf("focus: %w", err)
		}
		e.scope = scope
		return nil
	}

	// A filter splits at the last []: the segments up to it navigate to the
	// candidate nodes (the kept ones), the segments after it form the predicate
	// tested on each candidate. With no [], the candidates are the current scope
	// and the whole path is the predicate.
	nav, pred := splitAtLastIter(segs)
	if len(pred) == 0 {
		return fmt.Errorf("focus: a filter needs a field path after the iterate before the value")
	}
	predPath, err := resolvePath(pred, e.vars)
	if err != nil {
		return fmt.Errorf("focus: %w", err)
	}
	want, err := e.argValue(args[1])
	if err != nil {
		return fmt.Errorf("focus: %w", err)
	}
	candidates, err := e.walkFocus(e.scope, nav)
	if err != nil {
		return fmt.Errorf("focus: %w", err)
	}

	value := fmt.Sprint(want)
	var kept []any
	for _, c := range candidates {
		m, ok := c.(Doc)
		if !ok {
			continue
		}
		if got, ok := Get(m, predPath); ok && fmt.Sprint(got) == value {
			kept = append(kept, c)
		}
	}
	e.scope = kept
	return nil
}

// splitAtLastIter divides a focus path at its final [] segment: nav includes the
// iterate (its elements are the filter candidates), pred is the predicate path
// after it. With no iterate, nav is empty (candidates are the current scope) and
// pred is the whole path.
func splitAtLastIter(segs []pathSeg) (nav, pred []pathSeg) {
	last := -1
	for i, s := range segs {
		if s.kind == segIter {
			last = i
		}
	}
	return segs[:last+1], segs[last+1:]
}

// walkFocus applies path segments across a node set, flattening: .key/[N] map
// each node to a child, [] expands each list node into its elements.
func (e *interpreter) walkFocus(scope []any, segs []pathSeg) ([]any, error) {
	cur := scope
	for _, s := range segs {
		var next []any
		for _, node := range cur {
			children, err := e.focusStep(node, s)
			if err != nil {
				return nil, err
			}
			next = append(next, children...)
		}
		cur = next
	}
	return cur, nil
}

// focusStep maps one node through one segment to the children it contributes:
// .key/[N] yield the single addressed child if present, [] expands a list node
// into its elements (and errors when applied to a non-list).
func (e *interpreter) focusStep(node any, s pathSeg) ([]any, error) {
	switch s.kind {
	case segKey:
		name, err := resolveStr(s.key, e.vars)
		if err != nil {
			return nil, err
		}
		m, ok := node.(Doc)
		if !ok {
			return nil, nil
		}
		if v, ok := m[name]; ok {
			return []any{v}, nil
		}
		return nil, nil

	case segIndex:
		if list, ok := node.([]any); ok && s.index >= 0 && s.index < len(list) {
			return []any{list[s.index]}, nil
		}
		return nil, nil

	case segIter:
		list, ok := node.([]any)
		if !ok {
			return nil, fmt.Errorf("'[]' applied to a non-list")
		}
		return list, nil
	}
	return nil, nil
}

func (e *interpreter) execReset(args []Arg) error {
	if len(args) != 0 {
		return fmt.Errorf("reset: takes no args, got %d", len(args))
	}
	if err := e.ensureDecoded(); err != nil {
		return err
	}
	e.resetScope()
	return nil
}

// remove-doc — drop every in-scope doc from the buffer, then reset scope to the
// remaining stream.
func (e *interpreter) execRemoveDoc(args []Arg) error {
	if len(args) != 0 {
		return fmt.Errorf("remove-doc: takes no args, got %d", len(args))
	}
	if err := e.ensureDecoded(); err != nil {
		return err
	}

	dropped := map[uintptr]bool{}
	for _, node := range e.scope {
		if p, ok := docPtr(node); ok {
			dropped[p] = true
		}
	}

	kept := make([]Doc, 0, len(e.docs))
	for _, doc := range e.docs {
		if p, ok := docPtr(doc); ok && dropped[p] {
			continue
		}
		kept = append(kept, doc)
	}
	e.docs = kept
	e.resetScope()
	return nil
}

// docPtr returns a map's identity pointer, for matching focused nodes back to
// the documents that hold them (maps are not == comparable).
func docPtr(v any) (uintptr, bool) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Map {
		return rv.Pointer(), true
	}
	return 0, false
}

// execValueEdit handles the PATH VALUE verbs: resolve the path and the value (a
// bare var reference or a quoted string), then apply fn to every in-scope doc.
func (e *interpreter) execValueEdit(args []Arg, fn func(Doc, Path, any) error) error {
	if len(args) != 2 {
		return fmt.Errorf("want PATH VALUE, got %d args", len(args))
	}
	path, err := e.argPath(args[0])
	if err != nil {
		return err
	}
	val, err := e.argValue(args[1])
	if err != nil {
		return err
	}
	return e.applyOverScope(path, func(doc Doc, p Path) error {
		return fn(doc, p, val)
	})
}

// execPathEdit handles the PATH-only verbs (remove).
func (e *interpreter) execPathEdit(args []Arg, fn func(Doc, Path) error) error {
	if len(args) != 1 {
		return fmt.Errorf("want PATH, got %d args", len(args))
	}
	path, err := e.argPath(args[0])
	if err != nil {
		return err
	}
	return e.applyOverScope(path, fn)
}

// applyOverScope runs fn on every focused node, with the edit path relative to
// that node. A focused node that is not a map can't be edited.
func (e *interpreter) applyOverScope(path Path, fn func(Doc, Path) error) error {
	if err := e.ensureDecoded(); err != nil {
		return err
	}
	for _, node := range e.scope {
		m, ok := node.(Doc)
		if !ok {
			return fmt.Errorf("edit: focused node is not a map (focus into a document first)")
		}
		if err := fn(m, path); err != nil {
			return err
		}
	}
	return nil
}

// argPath resolves a path argument; any other arg kind is a usage error.
func (e *interpreter) argPath(a Arg) (Path, error) {
	if a.kind != argPath {
		return nil, fmt.Errorf("expected a path (.a.b)")
	}
	return resolvePath(a.path, e.vars)
}

// argValue resolves a value argument: a bare token is a variable reference
// (native type), a quoted token is a string.
func (e *interpreter) argValue(a Arg) (any, error) {
	switch a.kind {
	case argStr:
		return resolveStr(a.str, e.vars)
	case argVar:
		v, ok := e.vars[a.name]
		if !ok {
			return nil, fmt.Errorf("undefined var %q (a bare value is a variable reference; quote a string literal as %q)", a.name, a.name)
		}
		return v, nil
	default:
		return nil, fmt.Errorf("expected a value (a var reference or a \"string\"), got a path")
	}
}

// argString resolves an argument expected to be text (URL, filename): a quoted
// string, or a variable reference stringified.
func (e *interpreter) argString(a Arg) (string, error) {
	v, err := e.argValue(a)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(v), nil
}

// setRaw parks raw bytes in the buffer, marking the decoded view stale.
func (e *interpreter) setRaw(data []byte) {
	e.raw = data
	e.decoded = false
}

// ensureDecoded materializes the decoded document stream from raw bytes the
// first time an edit or emit needs it, resetting scope to the fresh stream.
func (e *interpreter) ensureDecoded() error {
	if e.decoded {
		return nil
	}
	docs, err := decodeStream(e.raw)
	if err != nil {
		return err
	}
	e.docs = docs
	e.decoded = true
	e.resetScope()
	return nil
}

// resetScope returns the scope to the whole document stream — a single node, the
// list of all docs, which a leading .[] iterates into the individual documents.
func (e *interpreter) resetScope() {
	docs := make([]any, len(e.docs))
	for i := range e.docs {
		docs[i] = e.docs[i]
	}
	e.scope = []any{docs}
}
