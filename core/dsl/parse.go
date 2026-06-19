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

		args := make([]any, len(tokens))
		for i, tok := range tokens {
			if args[i], err = resolve(tok, e.vars); err != nil {
				return nil, fmt.Errorf("line %d: %w", n+1, err)
			}
		}
		verb, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("line %d: verb must be text", n+1)
		}
		if err := e.exec(verb, args[1:]); err != nil {
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

func (e *engine) exec(verb string, args []any) error {
	switch verb {
	case "download":
		return e.execDownload(args)
	case "extract":
		return e.execExtract(args)
	case "emit":
		return e.execEmit(args)
	case "select":
		return e.execSelect(args)
	case "reset":
		return e.execReset(args)
	case "set":
		return e.execEdit(args, 2, func(doc Doc, p Path) error {
			return Set(doc, p, args[1])
		})
	case "set-if-absent":
		return e.execEdit(args, 2, func(doc Doc, p Path) error {
			if _, ok := Get(doc, p); ok {
				return nil
			}
			return Set(doc, p, args[1])
		})
	case "append":
		return e.execEdit(args, 2, func(doc Doc, p Path) error {
			return Append(doc, p, args[1], false)
		})
	case "append-if-absent":
		return e.execEdit(args, 2, func(doc Doc, p Path) error {
			return Append(doc, p, args[1], true)
		})
	case "remove":
		return e.execEdit(args, 1, func(doc Doc, p Path) error {
			return Remove(doc, p)
		})
	case "remove-doc":
		return e.execRemoveDoc(args)
	default:
		return fmt.Errorf("unknown verb %q", verb)
	}
}

// download URL — fetch into the buffer, replacing it with raw bytes.
func (e *engine) execDownload(args []any) error {
	if len(args) != 1 {
		return fmt.Errorf("download: want URL, got %d args", len(args))
	}
	url, err := argStr(args, 0)
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
func (e *engine) execExtract(args []any) error {
	if len(args) > 1 {
		return fmt.Errorf("extract: want [PATH], got %d args", len(args))
	}
	if e.decoded {
		return fmt.Errorf("extract: nothing to extract (no prior download)")
	}

	member := ""
	if len(args) == 1 {
		m, err := argStr(args, 0)
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
func (e *engine) execEmit(args []any) error {
	if len(args) != 1 {
		return fmt.Errorf("emit: want FILENAME, got %d args", len(args))
	}
	name, err := argStr(args, 0)
	if err != nil {
		return fmt.Errorf("emit: %w", err)
	}
	if err := e.ensureDecoded(); err != nil {
		return err
	}
	return emit(e.outDir, name, e.docs)
}

// select PATH VALUE — narrow scope to in-scope docs whose PATH equals VALUE.
func (e *engine) execSelect(args []any) error {
	if len(args) != 2 {
		return fmt.Errorf("select: want PATH VALUE, got %d args", len(args))
	}
	pathStr, err := argStr(args, 0)
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

	value := fmt.Sprint(args[1])
	var kept []int
	for _, idx := range e.scope {
		if got, ok := Get(e.docs[idx], path); ok && fmt.Sprint(got) == value {
			kept = append(kept, idx)
		}
	}
	e.scope = kept
	return nil
}

func (e *engine) execReset(args []any) error {
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
func (e *engine) execRemoveDoc(args []any) error {
	if len(args) != 0 {
		return fmt.Errorf("remove-doc: takes no args, got %d", len(args))
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

// execEdit applies fn to every in-scope doc, after validating arg count and
// parsing args[0] as the target path. The value arg (args[1], if any) is left
// untyped for fn — set/append assign it as-is, never coercing.
func (e *engine) execEdit(args []any, want int, fn func(Doc, Path) error) error {
	if len(args) != want {
		return fmt.Errorf("want %d args, got %d", want, len(args))
	}
	pathStr, err := argStr(args, 0)
	if err != nil {
		return err
	}
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

// argStr asserts a string argument. Every directive arg except the set/append
// value (paths, filenames, URLs) is text; a typed value there is a usage error.
func argStr(args []any, i int) (string, error) {
	if s, ok := args[i].(string); ok {
		return s, nil
	}
	return "", fmt.Errorf("argument %d must be text, got %T", i+1, args[i])
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
