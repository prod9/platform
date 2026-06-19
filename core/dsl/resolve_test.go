package dsl

import "testing"

// resolveString handles structural tokens (verb, path, URL, filename): bare is
// literal text, quoted interpolates to a string.
func TestResolveString(t *testing.T) {
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
			got, err := resolveString(tc.tok, vars)
			if err != nil {
				t.Fatalf("resolveString(%#v) error: %v", tc.tok, err)
			}
			if got != tc.want {
				t.Fatalf("resolveString(%#v) = %q, want %q", tc.tok, got, tc.want)
			}
		})
	}
}

func TestResolveStringErrors(t *testing.T) {
	vars := Vars{"y": "z"}
	cases := []struct {
		name string
		tok  Token
	}{
		{"undefined var", Token{`\(nope)`, true}},
		{"bare forgotten quote", Token{`\(y)`, false}}, // bare \( is a missing quote
		{"unterminated interp", Token{`\(y`, true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := resolveString(tc.tok, vars); err == nil {
				t.Fatalf("resolveString(%#v) expected error, got nil", tc.tok)
			}
		})
	}
}

// resolveValue handles the value position: a bare token is a variable reference
// (native type), a quoted token is a string literal/interpolation.
func TestResolveValue(t *testing.T) {
	vars := Vars{"y": "z", "count": 3, "on": true}

	cases := []struct {
		name string
		tok  Token
		want any
	}{
		{"bare ref string var", Token{"y", false}, "z"},
		{"bare ref int var", Token{"count", false}, 3},
		{"bare ref bool var", Token{"on", false}, true},
		{"quoted is string literal", Token{"off", true}, "off"},
		{"quoted interpolation stringifies", Token{`\(count)`, true}, "3"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveValue(tc.tok, vars)
			if err != nil {
				t.Fatalf("resolveValue(%#v) error: %v", tc.tok, err)
			}
			if got != tc.want {
				t.Fatalf("resolveValue(%#v) = %#v, want %#v", tc.tok, got, tc.want)
			}
		})
	}
}

func TestResolveValueErrors(t *testing.T) {
	vars := Vars{"y": "z"}
	cases := []struct {
		name string
		tok  Token
	}{
		{"bare undefined var", Token{"nope", false}},     // strict: no literal fallback
		{"bare interp not allowed", Token{`\(y)`, false}}, // quote it
		{"quoted undefined var", Token{`\(nope)`, true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := resolveValue(tc.tok, vars); err == nil {
				t.Fatalf("resolveValue(%#v) expected error, got nil", tc.tok)
			}
		})
	}
}
