package dsl

import (
	"fmt"
	"strings"
)

// Lex splits one directive line into tokens. Tokens are whitespace-separated; a
// double-quoted run groups a value containing spaces or '#'; an unquoted '#'
// starts a comment to end of line. Blank and comment-only lines yield no tokens.
//
// Escapes inside quotes are \" and \\. Other backslash sequences — notably \( —
// are preserved verbatim, since \(var) interpolation is layered on in D2.
func Lex(line string) ([]string, error) {
	var tokens []string

	for i, n := 0, len(line); i < n; {
		for i < n && isSpace(line[i]) {
			i++
		}
		if i >= n || line[i] == '#' {
			break
		}

		if line[i] == '"' {
			value, next, err := scanString(line, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, value)
			i = next
			continue
		}

		value, next := scanBare(line, i)
		tokens = append(tokens, value)
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

// scanString reads a double-quoted token starting at the opening quote. It
// resolves \" and \\, preserves other escapes verbatim, and returns the index
// just past the closing quote.
func scanString(line string, start int) (value string, next int, err error) {
	var b strings.Builder

	for i := start + 1; i < len(line); i++ {
		switch c := line[i]; c {
		case '\\':
			if i+1 >= len(line) {
				return "", 0, fmt.Errorf("dangling escape in string: %q", line[start:])
			}
			i++
			if esc := line[i]; esc == '"' || esc == '\\' {
				b.WriteByte(esc)
			} else {
				b.WriteByte('\\')
				b.WriteByte(esc)
			}
		case '"':
			return b.String(), i + 1, nil
		default:
			b.WriteByte(c)
		}
	}
	return "", 0, fmt.Errorf("unterminated string: %q", line[start:])
}
