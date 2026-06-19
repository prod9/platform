package dsl

import "testing"

func mustParts(t *testing.T, quoted string) []strPart {
	t.Helper()
	parts, next, err := scanString(quoted, 0)
	if err != nil {
		t.Fatalf("scanString(%q): %v", quoted, err)
	}
	if next != len(quoted) {
		t.Fatalf("scanString(%q): trailing input past %d", quoted, next)
	}
	return parts
}

// TestResolveStr exercises the string pipeline: scanString parses escapes and
// \(var) parts, resolveStr renders them against vars (interpolated values
// stringified).
func TestResolveStr(t *testing.T) {
	vars := Vars{"y": "z", "prefix": "cm", "version": "v1.2.3", "count": 3}

	cases := []struct {
		name   string
		quoted string
		want   string
	}{
		{"plain", `"off"`, "off"},
		{"empty", `""`, ""},
		// \\( must win over \( so the literal stays literal.
		{"literal escaped interp", `"\\(y)"`, `\(y)`},
		{"interp", `"\(y)"`, "z"},
		{"interp mid-string", `"\(prefix)-controller"`, "cm-controller"},
		{"interp in url", `"u/\(version)/install.yaml"`, "u/v1.2.3/install.yaml"},
		{"interp stringifies typed", `"\(count)"`, "3"},
		{"escaped backslash", `"x\\y"`, `x\y`},
		{"escaped quote", `"he said \"hi\""`, `he said "hi"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveStr(mustParts(t, tc.quoted), vars)
			if err != nil {
				t.Fatalf("resolveStr(%q): %v", tc.quoted, err)
			}
			if got != tc.want {
				t.Fatalf("resolveStr(%q) = %q, want %q", tc.quoted, got, tc.want)
			}
		})
	}
}

func TestResolveStrUndefinedVar(t *testing.T) {
	if _, err := resolveStr(mustParts(t, `"\(nope)"`), Vars{}); err == nil {
		t.Fatal("expected error for undefined var, got nil")
	}
}

func TestScanStringErrors(t *testing.T) {
	for _, in := range []string{`"unterminated`, `"dangling\`, `"bad \(interp"`} {
		if _, _, err := scanString(in, 0); err == nil {
			t.Errorf("scanString(%q) expected error, got nil", in)
		}
	}
}
