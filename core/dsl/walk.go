package dsl

import (
	"fmt"
	"slices"
)

// Get walks path from doc and returns the value it names, or ok=false if any
// step is missing. Lists decode as []any, maps as map[string]any.
func Get(doc map[string]any, path Path) (any, bool) {
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

// Set assigns value at path, creating missing intermediate maps along the way.
// It cannot fabricate list elements: a missing Index/Select intermediate is an
// error, since there is nothing to address.
func Set(doc map[string]any, path Path, value any) error {
	parent, last, err := walkToParent(doc, path, true)
	if err != nil {
		return err
	}
	return assign(parent, last, value)
}

// Remove deletes the field or list element at path. Removing a list element
// shortens the slice and writes the shortened list back to its container.
func Remove(doc map[string]any, path Path) error {
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
		m, ok := container.(map[string]any)
		if !ok {
			return fmt.Errorf("remove: cannot delete key %q from non-map", s.Name)
		}
		delete(m, s.Name)
		return nil

	case Index, Select:
		list, ok := container.([]any)
		if !ok {
			return fmt.Errorf("remove: cannot delete element from non-list")
		}
		i := stepIndex(list, s)
		if i < 0 {
			return fmt.Errorf("remove: element not found: %v", s)
		}
		shortened := append(list[:i:i], list[i+1:]...)
		return Set(doc, prefix, shortened)
	}
	return fmt.Errorf("remove: unknown step %v", last)
}

// Append adds value to the list at path, creating an empty list if path is
// absent. unique=true skips the append when value is already present.
func Append(doc map[string]any, path Path, value any, unique bool) error {
	existing, _ := Get(doc, path)
	list, _ := existing.([]any)

	if unique && slices.Contains(list, value) {
		return nil
	}
	return Set(doc, path, append(list, value))
}

// stepInto descends one step from node, reporting whether the target exists.
func stepInto(node any, s Step) (any, bool) {
	switch s := s.(type) {
	case Key:
		m, ok := node.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok := m[s.Name]
		return v, ok

	case Index, Select:
		list, ok := node.([]any)
		if !ok {
			return nil, false
		}
		i := stepIndex(list, s)
		if i < 0 {
			return nil, false
		}
		return list[i], true
	}
	return nil, false
}

// walkToParent returns the container holding path's final step, plus that step.
// With create set, missing intermediate map keys are created en route.
func walkToParent(doc map[string]any, path Path, create bool) (parent any, last Step, err error) {
	var node any = doc
	for _, s := range path[:len(path)-1] {
		next, ok := stepInto(node, s)
		if ok {
			node = next
			continue
		}

		key, isKey := s.(Key)
		if !create || !isKey {
			return nil, nil, fmt.Errorf("path step not found: %v", s)
		}
		m, ok := node.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("cannot create %q under non-map", key.Name)
		}
		child := map[string]any{}
		m[key.Name] = child
		node = child
	}
	return node, path[len(path)-1], nil
}

// assign writes value at last within parent.
func assign(parent any, last Step, value any) error {
	switch s := last.(type) {
	case Key:
		m, ok := parent.(map[string]any)
		if !ok {
			return fmt.Errorf("cannot set key %q on non-map", s.Name)
		}
		m[s.Name] = value
		return nil

	case Index, Select:
		list, ok := parent.([]any)
		if !ok {
			return fmt.Errorf("cannot set element on non-list")
		}
		i := stepIndex(list, s)
		if i < 0 {
			return fmt.Errorf("cannot set: element not found: %v", s)
		}
		list[i] = value
		return nil
	}
	return fmt.Errorf("unknown step %v", last)
}

// stepIndex resolves an Index or Select step against list to a concrete
// position, or -1 when out of range or unmatched.
func stepIndex(list []any, s Step) int {
	switch s := s.(type) {
	case Index:
		if s.N >= 0 && s.N < len(list) {
			return s.N
		}
		return -1

	case Select:
		for i, elem := range list {
			m, ok := elem.(map[string]any)
			if !ok {
				continue
			}
			if fmt.Sprint(m[s.Field]) == s.Value {
				return i
			}
		}
		return -1
	}
	return -1
}
