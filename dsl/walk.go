package dsl

import (
	"fmt"
	"slices"
)

// Doc is one decoded YAML document — and, recursively, any mapping node within
// it. A directive file edits a stream of these ([]Doc).
type Doc = map[string]any

// Vars is the \(var) interpolation table, sourced from platform.toml's
// [ops.vars]. Values keep their TOML type (string/int/bool): interpolation into
// a quoted string stringifies them, while a bare \(x) value preserves the type.
type Vars = map[string]any

// Get walks path from doc and returns the value it names, or ok=false if any
// step is missing. Lists decode as []any, maps as Doc.
func Get(doc Doc, path Path) (any, bool) {
	var node any = doc
	for _, s := range path {
		next, ok := stepInto(node, s)
		if !ok {
			return nil, false
		}
		node = next
	}
	return node, true
}

// Set assigns value at path, auto-vivifying the route: missing map keys become
// maps and missing Index slots extend the list (its element type follows the
// next step), so a deep scalar Set can build a nested structure from nothing.
func Set(doc Doc, path Path, value any) error {
	_, err := setPath(doc, path, value)
	return err
}

// setPath sets value at path within node, returning the node it should be bound
// to in its parent (the same map, or a possibly-reallocated list).
func setPath(node any, path Path, value any) (any, error) {
	if len(path) == 0 {
		return value, nil
	}

	switch s := path[0].(type) {
	case Key:
		m, ok := emptyOr(node, Doc{})
		if !ok {
			return nil, fmt.Errorf("set: cannot descend key %q into non-map", s.Name)
		}
		child, err := setPath(m[s.Name], path[1:], value)
		if err != nil {
			return nil, err
		}
		m[s.Name] = child
		return m, nil

	case Index:
		list, ok := emptyOr(node, []any(nil))
		if !ok {
			return nil, fmt.Errorf("set: cannot index into non-list")
		}
		for len(list) <= s.N {
			list = append(list, nil)
		}
		child, err := setPath(list[s.N], path[1:], value)
		if err != nil {
			return nil, err
		}
		list[s.N] = child
		return list, nil
	}
	return nil, fmt.Errorf("set: unknown step %T", path[0])
}

// emptyOr asserts node to T, substituting empty when node is absent (nil). It
// reports false only when node holds a concrete value of a conflicting type.
func emptyOr[T any](node any, empty T) (T, bool) {
	if node == nil {
		return empty, true
	}
	t, ok := node.(T)
	return t, ok
}

// Remove deletes the field or list element at path. Removing a list element
// shortens the slice and writes the shortened list back to its container.
func Remove(doc Doc, path Path) error {
	if len(path) == 0 {
		return fmt.Errorf("remove: empty path")
	}
	last := path[len(path)-1]
	prefix := path[:len(path)-1]

	container := any(doc)
	if len(prefix) > 0 {
		c, ok := Get(doc, prefix)
		if !ok {
			return fmt.Errorf("remove: path not found: %v", prefix)
		}
		container = c
	}

	switch s := last.(type) {
	case Key:
		m, ok := container.(Doc)
		if !ok {
			return fmt.Errorf("remove: cannot delete key %q from non-map", s.Name)
		}
		delete(m, s.Name)
		return nil

	case Index:
		list, ok := container.([]any)
		if !ok {
			return fmt.Errorf("remove: cannot delete element from non-list")
		}
		if s.N < 0 || s.N >= len(list) {
			return fmt.Errorf("remove: index %d out of range", s.N)
		}
		shortened := append(list[:s.N:s.N], list[s.N+1:]...)
		return Set(doc, prefix, shortened)
	}
	return fmt.Errorf("remove: unknown step %T", last)
}

// Append adds value to the list at path, creating an empty list if path is
// absent.
func Append(doc Doc, path Path, value any) error {
	existing, _ := Get(doc, path)
	list, _ := existing.([]any)
	return Set(doc, path, append(list, value))
}

// AppendIfAbsent appends value to the list at path only when it is not already
// present, leaving the document untouched on a hit.
func AppendIfAbsent(doc Doc, path Path, value any) error {
	existing, _ := Get(doc, path)
	if list, _ := existing.([]any); slices.Contains(list, value) {
		return nil
	}
	return Append(doc, path, value)
}

// stepInto descends one step from node, reporting whether the target exists.
func stepInto(node any, s Step) (any, bool) {
	switch s := s.(type) {
	case Key:
		m, ok := node.(Doc)
		if !ok {
			return nil, false
		}
		v, ok := m[s.Name]
		return v, ok

	case Index:
		list, ok := node.([]any)
		if !ok || s.N < 0 || s.N >= len(list) {
			return nil, false
		}
		return list[s.N], true
	}
	return nil, false
}
