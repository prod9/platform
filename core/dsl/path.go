package dsl

import "fmt"

// Path is a resolved selector to a single node within a document: a sequence of
// steps walked left to right. Edits (set/append/remove) address one node, so a
// Path holds only Key and Index — never the [] iterate, which belongs to focus.
type Path []Step

// Step is one segment of a Path. The set is closed: Key or Index.
type Step interface{ step() }

// Key selects a map entry by name: ".spec".
type Key struct{ Name string }

// Index selects a list element by position: "[0]".
type Index struct{ N int }

func (Key) step()   {}
func (Index) step() {}

// pathFromString lexes and parses a single edit path — a convenience for callers
// and tests holding a path as a string. It runs the real lexer and parser, not a
// separate scanner; an iterate ([]) is rejected, as in any edit path.
func pathFromString(s string) (Path, error) {
	toks, err := lexLine(s)
	if err != nil {
		return nil, err
	}
	if len(toks) == 0 || toks[0].kind != tDot {
		return nil, fmt.Errorf("path must start with '.': %q", s)
	}

	arg, next, err := parsePath(toks, 0)
	if err != nil {
		return nil, err
	}
	if next != len(toks) {
		return nil, fmt.Errorf("trailing tokens in path %q", s)
	}
	return resolvePath(arg.path, nil)
}
