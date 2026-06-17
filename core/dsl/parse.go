package dsl

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Apply runs a directive file against a decoded multi-document buffer, mutating
// the docs in place, and returns the resulting stream. Scope starts at the whole
// stream; select narrows it, reset widens it back.
//
// D1 implements the in-buffer verbs only. The I/O verbs (download, extract, emit)
// and \(var) interpolation arrive in D2; until then an unknown verb is an error.
func Apply(directives string, docs []map[string]any) ([]map[string]any, error) {
	e := &engine{buffer: docs}
	e.resetScope()

	for n, line := range strings.Split(directives, "\n") {
		tokens, err := Lex(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", n+1, err)
		}
		if len(tokens) == 0 {
			continue
		}
		if err := e.exec(tokens[0], tokens[1:]); err != nil {
			return nil, fmt.Errorf("line %d: %w", n+1, err)
		}
	}
	return e.buffer, nil
}

// engine carries the directive interpreter's state: the document buffer and the
// indices of the documents currently in scope.
type engine struct {
	buffer []map[string]any
	scope  []int
}

func (e *engine) exec(verb string, args []string) error {
	switch verb {
	case "select":
		return e.execSelect(args)
	case "reset":
		return e.execReset(args)
	case "set":
		return e.execEdit(args, 2, func(doc map[string]any, p Path) error {
			return Set(doc, p, scalar(args[1]))
		})
	case "set-if-absent":
		return e.execEdit(args, 2, func(doc map[string]any, p Path) error {
			if _, ok := Get(doc, p); ok {
				return nil
			}
			return Set(doc, p, scalar(args[1]))
		})
	case "append":
		return e.execEdit(args, 2, func(doc map[string]any, p Path) error {
			return Append(doc, p, scalar(args[1]), false)
		})
	case "append-if-absent":
		return e.execEdit(args, 2, func(doc map[string]any, p Path) error {
			return Append(doc, p, scalar(args[1]), true)
		})
	case "remove":
		return e.execEdit(args, 1, func(doc map[string]any, p Path) error {
			return Remove(doc, p)
		})
	case "remove-doc":
		return e.execRemoveDoc(args)
	default:
		return fmt.Errorf("unknown verb %q", verb)
	}
}

// select PATH VALUE — narrow scope to in-scope docs whose PATH equals VALUE.
func (e *engine) execSelect(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("select: want PATH VALUE, got %d args", len(args))
	}
	path, err := ParsePath(args[0])
	if err != nil {
		return err
	}

	value := args[1]
	var kept []int
	for _, idx := range e.scope {
		if got, ok := Get(e.buffer[idx], path); ok && fmt.Sprint(got) == value {
			kept = append(kept, idx)
		}
	}
	e.scope = kept
	return nil
}

func (e *engine) execReset(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("reset: takes no args, got %d", len(args))
	}
	e.resetScope()
	return nil
}

// remove-doc — drop every in-scope doc from the buffer, then reset scope to the
// remaining stream.
func (e *engine) execRemoveDoc(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("remove-doc: takes no args, got %d", len(args))
	}

	dropped := make(map[int]bool, len(e.scope))
	for _, idx := range e.scope {
		dropped[idx] = true
	}

	kept := make([]map[string]any, 0, len(e.buffer))
	for i, doc := range e.buffer {
		if !dropped[i] {
			kept = append(kept, doc)
		}
	}
	e.buffer = kept
	e.resetScope()
	return nil
}

// execEdit applies fn to every in-scope doc, after validating arg count and
// parsing args[0] as the target path.
func (e *engine) execEdit(args []string, want int, fn func(map[string]any, Path) error) error {
	if len(args) != want {
		return fmt.Errorf("want %d args, got %d", want, len(args))
	}
	path, err := ParsePath(args[0])
	if err != nil {
		return err
	}

	for _, idx := range e.scope {
		if err := fn(e.buffer[idx], path); err != nil {
			return err
		}
	}
	return nil
}

func (e *engine) resetScope() {
	e.scope = make([]int, len(e.buffer))
	for i := range e.buffer {
		e.scope[i] = i
	}
}

// scalar interprets a directive value token as a typed YAML scalar, so
// `set .spec.replicas 3` writes int 3, not the string "3".
func scalar(s string) any {
	if s == "" {
		return ""
	}
	var v any
	if err := yaml.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	return v
}
