package dsl

import (
	"reflect"
	"testing"
)

func TestParsePath(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want Path
	}{
		{"single key", ".kind", Path{Key{"kind"}}},
		{"nested keys", ".spec.replicas", Path{Key{"spec"}, Key{"replicas"}}},
		{"list index", ".spec.containers[0]", Path{Key{"spec"}, Key{"containers"}, Index{0}}},
		{"field select", ".spec.containers[name=ctl]",
			Path{Key{"spec"}, Key{"containers"}, Select{"name", "ctl"}}},
		{"select then key", ".spec.containers[name=ctl].image",
			Path{Key{"spec"}, Key{"containers"}, Select{"name", "ctl"}, Key{"image"}}},
		{"value with equals", ".a[k=x=y]", Path{Key{"a"}, Select{"k", "x=y"}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParsePath(tc.in)
			if err != nil {
				t.Fatalf("ParsePath(%q) error: %v", tc.in, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ParsePath(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}

func TestParsePathErrors(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"no leading dot", "spec.replicas"},
		{"unclosed bracket", ".spec.containers[0"},
		{"non-numeric index", ".spec.containers[x]"},
		{"empty key", ".spec..replicas"},
		{"empty field select", ".spec.containers[=x]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParsePath(tc.in); err == nil {
				t.Fatalf("ParsePath(%q) expected error, got nil", tc.in)
			}
		})
	}
}
