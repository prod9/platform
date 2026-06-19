package dsl

import (
	"fmt"
	"strconv"
	"strings"
)

// Path is a parsed selector into a decoded YAML document: a sequence of steps
// walked left to right. Built by ParsePath from the dotted directive syntax.
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

// ParsePath compiles the dotted path syntax into a Path. A path is one or more
// dot-prefixed keys, each optionally followed by "[...]" index or "[field=value]"
// field-select suffixes: ".spec.containers[name=ctl].image".
func ParsePath(s string) (Path, error) {
	if s == "" || s[0] != '.' {
		return nil, fmt.Errorf("path must start with '.': %q", s)
	}

	var path Path
	rest := s
	for len(rest) > 0 {
		rest = rest[1:] // consume the leading '.'

		name, after, err := scanKey(rest)
		if err != nil {
			return nil, fmt.Errorf("%w in path %q", err, s)
		}
		if name == "" {
			return nil, fmt.Errorf("empty key in path %q", s)
		}
		path = append(path, Key{Name: name})
		rest = after

		for len(rest) > 0 && rest[0] == '[' {
			step, after, err := scanBracket(rest, s)
			if err != nil {
				return nil, err
			}
			path = append(path, step)
			rest = after
		}
	}
	return path, nil
}

// scanKey reads a map key. A quoted key (`"…"`, jq-style) is taken verbatim so
// dotted/slashed names like annotations survive; an unquoted key runs up to the
// next '.' or '[' boundary.
func scanKey(s string) (name, rest string, err error) {
	if len(s) > 0 && s[0] == '"' {
		end := strings.IndexByte(s[1:], '"')
		if end < 0 {
			return "", "", fmt.Errorf("unclosed quoted key")
		}
		return s[1 : 1+end], s[1+end+1:], nil
	}

	if i := strings.IndexAny(s, ".["); i >= 0 {
		return s[:i], s[i:], nil
	}
	return s, "", nil
}

// scanBracket parses one "[...]" suffix into an Index or Select. full is the
// whole path, carried only for error messages.
func scanBracket(s, full string) (Step, string, error) {
	end := strings.IndexByte(s, ']')
	if end < 0 {
		return nil, "", fmt.Errorf("unclosed '[' in path %q", full)
	}
	inner, rest := s[1:end], s[end+1:]

	if field, value, ok := strings.Cut(inner, "="); ok {
		if field == "" {
			return nil, "", fmt.Errorf("empty field-select in path %q", full)
		}
		return Select{Field: field, Value: value}, rest, nil
	}

	n, err := strconv.Atoi(inner)
	if err != nil {
		return nil, "", fmt.Errorf("invalid list index %q in path %q", inner, full)
	}
	if n < 0 {
		return nil, "", fmt.Errorf("negative list index %d in path %q", n, full)
	}
	return Index{N: n}, rest, nil
}
