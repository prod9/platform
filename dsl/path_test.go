package dsl

import (
	"reflect"
	"testing"
)

func TestPathFromString(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want Path
	}{
		{"single key", ".kind", Path{Key{"kind"}}},
		{"nested keys", ".spec.replicas", Path{Key{"spec"}, Key{"replicas"}}},
		{"list index glued", ".spec.containers[0]", Path{Key{"spec"}, Key{"containers"}, Index{0}}},
		{"index then key", ".spec.containers[0].image",
			Path{Key{"spec"}, Key{"containers"}, Index{0}, Key{"image"}}},
		{"leading index", ".[0]", Path{Index{0}}},
		{"quoted key with dots", `.metadata.annotations."svc.io/fw-id"`,
			Path{Key{"metadata"}, Key{"annotations"}, Key{"svc.io/fw-id"}}},
		{"quoted key then key", `."a.b".c`, Path{Key{"a.b"}, Key{"c"}}},
		{"quoted key then index", `."a.b"[0]`, Path{Key{"a.b"}, Index{0}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := pathFromString(tc.in)
			if err != nil {
				t.Fatalf("pathFromString(%q) error: %v", tc.in, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("pathFromString(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}

func TestPathFromStringErrors(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"no leading dot", "spec.replicas"},
		{"unclosed bracket", ".spec.containers[0"},
		{"non-numeric index", ".spec.containers[x]"},
		{"empty key", ".spec..replicas"},
		{"trailing dot", ".spec."},
		{"iterate not allowed in edit path", ".spec.containers[]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := pathFromString(tc.in); err == nil {
				t.Fatalf("pathFromString(%q) expected error, got nil", tc.in)
			}
		})
	}
}
