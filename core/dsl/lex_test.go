package dsl

import (
	"reflect"
	"testing"
)

// summarize renders a token stream as compact strings for comparison: structural
// tokens as their glyph, idents as text, strings as "…" with \(ref) for interps.
func summarize(toks []token) []string {
	var out []string
	for _, t := range toks {
		switch t.kind {
		case tDot:
			out = append(out, ".")
		case tLBrack:
			out = append(out, "[")
		case tRBrack:
			out = append(out, "]")
		case tEq:
			out = append(out, "=")
		case tIdent:
			out = append(out, t.text)
		case tStr:
			s := `"`
			for _, p := range t.parts {
				if p.ref != "" {
					s += `\(` + p.ref + `)`
				} else {
					s += p.text
				}
			}
			out = append(out, s+`"`)
		}
	}
	return out
}

func TestLexLine(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"simple", `set .kind "DaemonSet"`, []string{"set", ".", "kind", `"DaemonSet"`}},
		{"path with index", "set .a[0].b foo", []string{"set", ".", "a", "[", "0", "]", ".", "b", "foo"}},
		{"quoted key", `set .m.a."x.y" v`, []string{"set", ".", "m", ".", "a", ".", `"x.y"`, "v"}},
		{"field select", "select .c[name=ctl] x", []string{"select", ".", "c", "[", "name", "=", "ctl", "]", "x"}},
		{"value with equals", "select .a[k=x=y] z", []string{"select", ".", "a", "[", "k", "=", "x=y", "]", "z"}},
		{"inline comment", "set .x 1 # note", []string{"set", ".", "x", "1"}},
		{"full-line comment", "# just a comment", nil},
		{"blank", "   ", nil},
		{"quoted hash and space", `emit "a #b.yaml"`, []string{"emit", `"a #b.yaml"`}},
		{"interp parts", `download "u/\(v)/x"`, []string{"download", `"u/\(v)/x"`}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			toks, err := lexLine(tc.in)
			if err != nil {
				t.Fatalf("lexLine(%q) error: %v", tc.in, err)
			}
			if got := summarize(toks); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("lexLine(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}

// TestLexSpaced checks the whitespace-adjacency flags the parser relies on to
// bound arguments: a path's interior tokens are contiguous, the next arg leads
// with a spaced token.
func TestLexSpaced(t *testing.T) {
	toks, err := lexLine(`set .a.b "v"`)
	if err != nil {
		t.Fatalf("lexLine error: %v", err)
	}
	// tokens: set(spaced) .(spaced) a b(false) .(false) ... wait: set . a . b "v"
	want := []bool{true, true, false, false, false, true} // set, ., a, ., b, "v"
	got := make([]bool, len(toks))
	for i, tk := range toks {
		got[i] = tk.spaced
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("spaced flags = %v, want %v (tokens %v)", got, want, summarize(toks))
	}
}

func TestLexErrors(t *testing.T) {
	for _, in := range []string{`set .a "unterminated`, `set .a "dangling\`, "set .a[0"} {
		if _, err := lexLine(in); err == nil {
			t.Errorf("lexLine(%q) expected error, got nil", in)
		}
	}
}
