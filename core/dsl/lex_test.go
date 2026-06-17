package dsl

import (
	"reflect"
	"testing"
)

func TestLex(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"simple", "set .kind DaemonSet", []string{"set", ".kind", "DaemonSet"}},
		{"tabs", "set\t.x\t1", []string{"set", ".x", "1"}},
		{"quoted spaces", `emit "a b.yaml"`, []string{"emit", "a b.yaml"}},
		{"inline comment", "set .x 1 # note", []string{"set", ".x", "1"}},
		{"full-line comment", "# just a comment", nil},
		{"blank", "   ", nil},
		{"quoted hash", `append .a "--x=#y"`, []string{"append", ".a", "--x=#y"}},
		{"escaped quote", `set .a "he said \"hi\""`, []string{"set", ".a", `he said "hi"`}},
		{"escaped backslash", `set .a "x\\y"`, []string{"set", ".a", `x\y`}},
		{"interp preserved", `download "u/\(v)/x"`, []string{"download", `u/\(v)/x`}},
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
