package dsl

import "fmt"

// Path is a resolved selector into a decoded YAML document: a sequence of steps
// walked left to right. The parser builds it from path tokens (see parsePath).
type Path []Step

// Step is one segment of a Path. The set is closed: Key, Index, or Select.
type Step interface{ step() }

// Key selects a map entry by name: ".spec".
type Key struct{ Name string }

// Index selects a list element by position: "[0]".
type Index struct{ N int }

// Select selects the list element whose Field equals Value: "[name=ctl]". It is
// version-robust where Index is not, since upstream reorders lists between releases.
type Select struct {
	Field string
	Value string
}

func (Key) step()    {}
func (Index) step()  {}
func (Select) step() {}

// pathFromString lexes and parses a single path expression — a convenience for
// callers and tests holding a path as a string rather than directive tokens. It
// runs the real lexer and parser, not a separate scanner.
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
