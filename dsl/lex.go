package dsl

import (
	"fmt"
	"strings"
)

// tokKind enumerates the directive language's lexical tokens.
type tokKind int

const (
	tDot    tokKind = iota // .
	tLBrack                // [
	tRBrack                // ]
	tIdent                 // bare word (verb, path key, var name)
	tStr                   // "quoted string" — carries interpolation parts
)

// strPart is one piece of a quoted string: literal text, or a \(ref) variable
// interpolation when ref is non-empty.
type strPart struct {
	text string
	ref  string
}

// token is one lexed unit. spaced records whether whitespace (or line start)
// preceded it — the parser uses that to find argument boundaries, since a path's
// segments are contiguous while arguments are whitespace-separated.
type token struct {
	kind   tokKind
	text   string    // tIdent
	parts  []strPart // tStr
	spaced bool
}

func (t token) describe() string {
	switch t.kind {
	case tDot:
		return "'.'"
	case tLBrack:
		return "'['"
	case tRBrack:
		return "']'"
	case tStr:
		return "a string"
	default:
		return fmt.Sprintf("%q", t.text)
	}
}

// lexLine tokenizes one directive line. Whitespace separates tokens (and is
// recorded via spaced); '#' starts a comment to end of line. A bracket holds an
// integer index ([0]) or nothing ([] iterate); there is no field-select form.
func lexLine(line string) ([]token, error) {
	var toks []token
	spaced := true

	for i := 0; i < len(line); {
		switch c := line[i]; c {
		case ' ', '\t', '\r':
			spaced = true
			i++

		case '#':
			return toks, nil

		case '.':
			toks = append(toks, token{kind: tDot, spaced: spaced})
			spaced = false
			i++

		case '[':
			toks = append(toks, token{kind: tLBrack, spaced: spaced})
			spaced = false
			i++

			start := i
			for i < len(line) && line[i] >= '0' && line[i] <= '9' {
				i++
			}
			if i > start {
				toks = append(toks, token{kind: tIdent, text: line[start:i]})
			}
			if i >= len(line) || line[i] != ']' {
				return nil, fmt.Errorf("'[' wants an integer index or '[]', then ']'")
			}
			toks = append(toks, token{kind: tRBrack})
			i++

		case ']':
			toks = append(toks, token{kind: tRBrack, spaced: spaced})
			spaced = false
			i++

		case '"':
			parts, next, err := scanString(line, i)
			if err != nil {
				return nil, err
			}
			toks = append(toks, token{kind: tStr, parts: parts, spaced: spaced})
			spaced = false
			i = next

		default:
			start := i
			for i < len(line) && !structural(line[i]) {
				i++
			}
			toks = append(toks, token{kind: tIdent, text: line[start:i], spaced: spaced})
			spaced = false
		}
	}
	return toks, nil
}

func structural(c byte) bool {
	switch c {
	case ' ', '\t', '\r', '#', '.', '[', ']', '"':
		return true
	}
	return false
}

// scanString reads a double-quoted string starting at s[start], returning its
// interpolation parts and the index past the closing quote. \\ and \" are
// literal escapes; \(name) is a variable interpolation; \\( therefore stays a
// literal backslash followed by '(' (the escape-ordering case).
func scanString(s string, start int) ([]strPart, int, error) {
	var parts []strPart
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			parts = append(parts, strPart{text: lit.String()})
			lit.Reset()
		}
	}

	for i := start + 1; i < len(s); {
		switch c := s[i]; c {
		case '"':
			flush()
			return parts, i + 1, nil

		case '\\':
			if i+1 >= len(s) {
				return nil, 0, fmt.Errorf("dangling escape in string")
			}
			switch n := s[i+1]; n {
			case '\\':
				lit.WriteByte('\\')
				i += 2
			case '"':
				lit.WriteByte('"')
				i += 2
			case '(':
				end := strings.IndexByte(s[i+2:], ')')
				if end < 0 {
					return nil, 0, fmt.Errorf("unterminated \\( in string")
				}
				flush()
				parts = append(parts, strPart{ref: s[i+2 : i+2+end]})
				i += 2 + end + 1
			default:
				lit.WriteByte('\\')
				lit.WriteByte(n)
				i += 2
			}

		default:
			lit.WriteByte(c)
			i++
		}
	}
	return nil, 0, fmt.Errorf("unterminated string")
}

// resolveStr renders a string's parts with vars, stringifying interpolated
// values. An undefined \(ref) is a hard error, never a silent blank.
func resolveStr(parts []strPart, vars Vars) (string, error) {
	var b strings.Builder
	for _, p := range parts {
		if p.ref == "" {
			b.WriteString(p.text)
			continue
		}
		v, ok := vars[p.ref]
		if !ok {
			return "", fmt.Errorf("undefined var \\(%s)", p.ref)
		}
		fmt.Fprint(&b, v)
	}
	return b.String(), nil
}
