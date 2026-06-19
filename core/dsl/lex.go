package dsl

import (
	"fmt"
	"strings"
)

// Token is one lexed directive token. Quoted records whether it came from a
// double-quoted run: quoting is what licenses \(var) interpolation and escape
// resolution, both of which happen later in resolve() — not here.
type Token struct {
	Value  string
	Quoted bool
}

// Lex splits one directive line into tokens. Tokens are whitespace-separated; a
// double-quoted run groups a value containing spaces or '#'; an unquoted '#'
// starts a comment to end of line. Blank and comment-only lines yield no tokens.
//
// Quoted tokens keep their escapes (\\, \", \() verbatim — Lex only finds token
// boundaries. Escape and \(var) resolution is deferred to resolve(), because
// telling \\( (a literal) from \( (an interpolation) is undecidable once \\ has
// already been collapsed to \.
func Lex(line string) ([]Token, error) {
	var tokens []Token

	for i, n := 0, len(line); i < n; {
		for i < n && isSpace(line[i]) {
			i++
		}
		if i >= n || line[i] == '#' {
			break
		}

		if line[i] == '"' {
			raw, next, err := scanString(line, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, Token{raw, true})
			i = next
			continue
		}

		value, next := scanBare(line, i)
		tokens = append(tokens, Token{value, false})
		i = next
	}
	return tokens, nil
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' }

// scanBare reads an unquoted token, ending at whitespace or an unquoted '#'.
func scanBare(line string, start int) (value string, next int) {
	i := start
	for i < len(line) && !isSpace(line[i]) && line[i] != '#' {
		i++
	}
	return line[start:i], i
}

// scanString reads a double-quoted token starting at the opening quote and
// returns its raw inner content with escapes intact. It honors \\ and \" only
// enough not to terminate early on an escaped quote; the bytes themselves are
// preserved for resolve() to interpret.
func scanString(line string, start int) (raw string, next int, err error) {
	for i := start + 1; i < len(line); i++ {
		switch line[i] {
		case '\\':
			if i+1 >= len(line) {
				return "", 0, fmt.Errorf("dangling escape in string: %q", line[start:])
			}
			i++ // skip the escaped char so a \" does not close the string
		case '"':
			return line[start+1 : i], i + 1, nil
		}
	}
	return "", 0, fmt.Errorf("unterminated string: %q", line[start:])
}

// resolve turns a lexed token into its value. A quoted token is always a string
// (escapes + \(var) interpolated as text — this is how you force a string). A
// bare token that is exactly one \(name) reference resolves to that var's native
// type (string/int/bool), so a typed [ops.vars] value reaches set unchanged. Any
// other bare \( is a forgotten-quote error; a plain bare token is its literal text.
func resolve(tok Token, vars Vars) (any, error) {
	if tok.Quoted {
		return interpolate(tok.Value, vars)
	}

	if name, ok := soleVarRef(tok.Value); ok {
		val, set := vars[name]
		if !set {
			return nil, fmt.Errorf("undefined var \\(%s)", name)
		}
		return val, nil
	}
	if strings.Contains(tok.Value, `\(`) {
		return nil, fmt.Errorf("bare token %q contains \\( — quote it to interpolate", tok.Value)
	}
	return tok.Value, nil
}

// soleVarRef reports whether v is exactly one \(name) reference (no surrounding
// text, no nesting), returning the bare name.
func soleVarRef(v string) (string, bool) {
	if len(v) < 4 || !strings.HasPrefix(v, `\(`) || !strings.HasSuffix(v, ")") {
		return "", false
	}
	name := v[2 : len(v)-1]
	if name == "" || strings.ContainsAny(name, `()\`) {
		return "", false
	}
	return name, true
}

// interpolate resolves escapes and \(var) references in one left-to-right pass,
// so \\( is consumed as an escaped backslash before its '(' can read as an
// interpolation. \(name) expands to vars[name]; an undefined name is a hard
// error, never a silent blank.
func interpolate(s string, vars Vars) (string, error) {
	var b strings.Builder

	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			b.WriteByte(s[i])
			continue
		}
		if i+1 >= len(s) {
			return "", fmt.Errorf("dangling escape in %q", s)
		}

		switch n := s[i+1]; n {
		case '\\':
			b.WriteByte('\\')
			i++
		case '"':
			b.WriteByte('"')
			i++
		case '(':
			rel := strings.IndexByte(s[i+2:], ')')
			if rel < 0 {
				return "", fmt.Errorf("unterminated \\( in %q", s)
			}
			name := s[i+2 : i+2+rel]
			val, ok := vars[name]
			if !ok {
				return "", fmt.Errorf("undefined var \\(%s)", name)
			}
			fmt.Fprint(&b, val)
			i += 2 + rel
		default:
			b.WriteByte('\\')
			b.WriteByte(n)
			i++
		}
	}
	return b.String(), nil
}
