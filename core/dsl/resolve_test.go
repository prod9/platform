package dsl

import "testing"

func TestResolve(t *testing.T) {
	vars := Vars{"y": "z", "prefix": "cm", "version": "v1.2.3"}

	cases := []struct {
		name string
		tok  Token
		want string
	}{
		// The escape-ordering case — \\( must win over \( so the literal stays literal.
		{"literal escaped interp", Token{`\\(y)`, true}, `\(y)`},
		{"interp", Token{`\(y)`, true}, "z"},
		{"interp mid-string", Token{`\(prefix)-controller`, true}, "cm-controller"},
		{"interp in url", Token{`u/\(version)/install.yaml`, true}, "u/v1.2.3/install.yaml"},
		{"escaped backslash", Token{`x\\y`, true}, `x\y`},
		{"escaped quote", Token{`he said \"hi\"`, true}, `he said "hi"`},
		{"bare plain", Token{"DaemonSet", false}, "DaemonSet"},
		{"bare with backslash not interp", Token{`x\\y`, false}, `x\\y`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolve(tc.tok, vars)
			if err != nil {
				t.Fatalf("resolve(%#v) error: %v", tc.tok, err)
			}
			if got != tc.want {
				t.Fatalf("resolve(%#v) = %q, want %q", tc.tok, got, tc.want)
			}
		})
	}
}

func TestResolveErrors(t *testing.T) {
	vars := Vars{"y": "z"}

	cases := []struct {
		name string
		tok  Token
	}{
		{"undefined var", Token{`\(nope)`, true}},
		{"forgotten quote", Token{`\(y)`, false}}, // bare \( is almost certainly a missing quote
		{"unterminated interp", Token{`\(y`, true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := resolve(tc.tok, vars); err == nil {
				t.Fatalf("resolve(%#v) expected error, got nil", tc.tok)
			}
		})
	}
}
