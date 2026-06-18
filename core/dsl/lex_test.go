package dsl

import (
	"reflect"
	"testing"
)

func TestLex(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []Token
	}{
		{"simple", "set .kind DaemonSet", []Token{{"set", false}, {".kind", false}, {"DaemonSet", false}}},
		{"tabs", "set\t.x\t1", []Token{{"set", false}, {".x", false}, {"1", false}}},
		{"quoted spaces", `emit "a b.yaml"`, []Token{{"emit", false}, {"a b.yaml", true}}},
		{"inline comment", "set .x 1 # note", []Token{{"set", false}, {".x", false}, {"1", false}}},
		{"full-line comment", "# just a comment", nil},
		{"blank", "   ", nil},
		{"quoted hash", `append .a "--x=#y"`, []Token{{"append", false}, {".a", false}, {"--x=#y", true}}},
		// Quoted tokens keep their escapes raw — resolution happens in resolve(), not Lex.
		{"escaped quote raw", `set .a "he said \"hi\""`, []Token{{"set", false}, {".a", false}, {`he said \"hi\"`, true}}},
		{"escaped backslash raw", `set .a "x\\y"`, []Token{{"set", false}, {".a", false}, {`x\\y`, true}}},
		{"interp raw", `download "u/\(v)/x"`, []Token{{"download", false}, {`u/\(v)/x`, true}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Lex(tc.in)
			if err != nil {
				t.Fatalf("Lex(%q) error: %v", tc.in, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Lex(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}

func TestLexErrors(t *testing.T) {
	for _, in := range []string{`set .a "unterminated`, `set .a "dangling\`} {
		if _, err := Lex(in); err == nil {
			t.Errorf("Lex(%q) expected error, got nil", in)
		}
	}
}
