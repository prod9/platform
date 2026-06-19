package dsl

import (
	"fmt"
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

// Apply runs a directive file against the buffer described by opts and returns
// the resulting decoded stream. Scope starts at the whole stream; select
// narrows it, reset widens it back. download/extract replace the buffer with raw
// bytes, decoded lazily the next time an edit or emit needs documents.
func Apply(directives string, opts Options) ([]Doc, error) {
	e := &engine{
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

	for n, line := range strings.Split(directives, "\n") {
		tokens, err := Lex(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", n+1, err)
		}
		if len(tokens) == 0 {
			continue
		}

		verb, err := resolveString(tokens[0], e.vars)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", n+1, err)
		}
		if err := e.exec(verb, tokens[1:]); err != nil {
			return nil, fmt.Errorf("line %d: %w", n+1, err)
		}
	}

	if err := e.ensureDecoded(); err != nil {
		return nil, err
	}
	return e.docs, nil
}

// engine carries the directive interpreter's state. The buffer is two-state:
// raw holds undecoded bytes after download/extract; docs holds the decoded
// stream once an edit or emit forces a decode. decoded says which is live.
type engine struct {
	raw     []byte
	docs    []Doc
	decoded bool
	scope   []int

	vars   Vars
	outDir string
	fetch  func(url string) ([]byte, error)
}

func (e *engine) exec(verb string, toks []Token) error {
	switch verb {
	case "download":
		return e.execDownload(toks)
	case "extract":
		return e.execExtract(toks)
	case "emit":
		return e.execEmit(toks)
	case "select":
		return e.execSelect(toks)
	case "reset":
		return e.execReset(toks)
	case "set":
		return e.execValueEdit(toks, func(doc Doc, p Path, v any) error {
			return Set(doc, p, v)
		})
	case "set-if-absent":
		return e.execValueEdit(toks, func(doc Doc, p Path, v any) error {
			if _, ok := Get(doc, p); ok {
				return nil
			}
			return Set(doc, p, v)
		})
	case "append":
		return e.execValueEdit(toks, func(doc Doc, p Path, v any) error {
			return Append(doc, p, v, false)
		})
	case "append-if-absent":
		return e.execValueEdit(toks, func(doc Doc, p Path, v any) error {
			return Append(doc, p, v, true)
		})
	case "remove":
		return e.execPathEdit(toks, func(doc Doc, p Path) error {
			return Remove(doc, p)
		})
	case "remove-doc":
		return e.execRemoveDoc(toks)
	default:
		return fmt.Errorf("unknown verb %q", verb)
	}
}

// download URL — fetch into the buffer, replacing it with raw bytes.
func (e *engine) execDownload(toks []Token) error {
	if len(toks) != 1 {
		return fmt.Errorf("download: want URL, got %d args", len(toks))
	}
	url, err := resolveString(toks[0], e.vars)
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
func (e *engine) execExtract(toks []Token) error {
	if len(toks) > 1 {
		return fmt.Errorf("extract: want [PATH], got %d args", len(toks))
	}
	if e.decoded {
		return fmt.Errorf("extract: nothing to extract (no prior download)")
	}

	member := ""
	if len(toks) == 1 {
		m, err := resolveString(toks[0], e.vars)
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
func (e *engine) execEmit(toks []Token) error {
	if len(toks) != 1 {
		return fmt.Errorf("emit: want FILENAME, got %d args", len(toks))
	}
	name, err := resolveString(toks[0], e.vars)
	if err != nil {
		return fmt.Errorf("emit: %w", err)
	}
	if err := e.ensureDecoded(); err != nil {
		return err
	}
	return emit(e.outDir, name, e.docs)
}

// select PATH VALUE — narrow scope to in-scope docs whose PATH equals VALUE.
func (e *engine) execSelect(toks []Token) error {
	if len(toks) != 2 {
		return fmt.Errorf("select: want PATH VALUE, got %d args", len(toks))
	}
	pathStr, err := resolveString(toks[0], e.vars)
	if err != nil {
		return fmt.Errorf("select: %w", err)
	}
	want, err := resolveValue(toks[1], e.vars)
	if err != nil {
		return fmt.Errorf("select: %w", err)
	}
	if err := e.ensureDecoded(); err != nil {
		return err
	}
	path, err := ParsePath(pathStr)
	if err != nil {
		return err
	}

	value := fmt.Sprint(want)
	var kept []int
	for _, idx := range e.scope {
		if got, ok := Get(e.docs[idx], path); ok && fmt.Sprint(got) == value {
			kept = append(kept, idx)
		}
	}
	e.scope = kept
	return nil
}

func (e *engine) execReset(toks []Token) error {
	if len(toks) != 0 {
		return fmt.Errorf("reset: takes no args, got %d", len(toks))
	}
	if err := e.ensureDecoded(); err != nil {
		return err
	}
	e.resetScope()
	return nil
}

// remove-doc — drop every in-scope doc from the buffer, then reset scope to the
// remaining stream.
func (e *engine) execRemoveDoc(toks []Token) error {
	if len(toks) != 0 {
		return fmt.Errorf("remove-doc: takes no args, got %d", len(toks))
	}
	if err := e.ensureDecoded(); err != nil {
		return err
	}

	dropped := make(map[int]bool, len(e.scope))
	for _, idx := range e.scope {
		dropped[idx] = true
	}

	kept := make([]Doc, 0, len(e.docs))
	for i, doc := range e.docs {
		if !dropped[i] {
			kept = append(kept, doc)
		}
	}
	e.docs = kept
	e.resetScope()
	return nil
}

// execValueEdit handles the PATH VALUE verbs: resolve the path (a structural
// string) and the value (a bare var reference or a quoted string), then apply fn
// to every in-scope doc.
func (e *engine) execValueEdit(toks []Token, fn func(Doc, Path, any) error) error {
	if len(toks) != 2 {
		return fmt.Errorf("want PATH VALUE, got %d args", len(toks))
	}
	pathStr, err := resolveString(toks[0], e.vars)
	if err != nil {
		return err
	}
	val, err := resolveValue(toks[1], e.vars)
	if err != nil {
		return err
	}
	return e.applyOverScope(pathStr, func(doc Doc, p Path) error {
		return fn(doc, p, val)
	})
}

// execPathEdit handles the PATH-only verbs (remove).
func (e *engine) execPathEdit(toks []Token, fn func(Doc, Path) error) error {
	if len(toks) != 1 {
		return fmt.Errorf("want PATH, got %d args", len(toks))
	}
	pathStr, err := resolveString(toks[0], e.vars)
	if err != nil {
		return err
	}
	return e.applyOverScope(pathStr, fn)
}

// applyOverScope parses pathStr and runs fn on every in-scope doc.
func (e *engine) applyOverScope(pathStr string, fn func(Doc, Path) error) error {
	if err := e.ensureDecoded(); err != nil {
		return err
	}
	path, err := ParsePath(pathStr)
	if err != nil {
		return err
	}

	for _, idx := range e.scope {
		if err := fn(e.docs[idx], path); err != nil {
			return err
		}
	}
	return nil
}

// setRaw parks raw bytes in the buffer, marking the decoded view stale.
func (e *engine) setRaw(data []byte) {
	e.raw = data
	e.decoded = false
}

// ensureDecoded materializes the decoded document stream from raw bytes the
// first time an edit or emit needs it, resetting scope to the fresh stream.
func (e *engine) ensureDecoded() error {
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

func (e *engine) resetScope() {
	e.scope = make([]int, len(e.docs))
	for i := range e.docs {
		e.scope[i] = i
	}
}
