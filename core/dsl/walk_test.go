package dsl

import (
	"reflect"
	"testing"
)

func sampleDoc() map[string]any {
	return map[string]any{
		"kind": "Deployment",
		"spec": map[string]any{
			"replicas": 3,
			"containers": []any{
				map[string]any{"name": "ctl", "args": []any{"--a"}},
				map[string]any{"name": "side"},
			},
		},
	}
}

func mustPath(t *testing.T, s string) Path {
	t.Helper()
	p, err := ParsePath(s)
	if err != nil {
		t.Fatalf("ParsePath(%q): %v", s, err)
	}
	return p
}

func TestGet(t *testing.T) {
	d := sampleDoc()
	cases := []struct {
		path string
		want any
	}{
		{".kind", "Deployment"},
		{".spec.replicas", 3},
		{".spec.containers[0].name", "ctl"},
		{".spec.containers[name=side].name", "side"},
		{".spec.containers[name=ctl].args[0]", "--a"},
	}

	for _, tc := range cases {
		got, ok := Get(d, mustPath(t, tc.path))
		if !ok {
			t.Errorf("Get(%q) not found", tc.path)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Get(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestGetMissing(t *testing.T) {
	d := sampleDoc()
	for _, p := range []string{".nope", ".spec.nope", ".spec.containers[5]", ".spec.containers[name=zzz]"} {
		if _, ok := Get(d, mustPath(t, p)); ok {
			t.Errorf("Get(%q) expected not found", p)
		}
	}
}

func TestSet(t *testing.T) {
	d := sampleDoc()

	if err := Set(d, mustPath(t, ".spec.replicas"), 5); err != nil {
		t.Fatal(err)
	}
	if got, _ := Get(d, mustPath(t, ".spec.replicas")); got != 5 {
		t.Errorf("replicas = %v, want 5", got)
	}

	// missing intermediate maps are created
	if err := Set(d, mustPath(t, ".metadata.labels.app"), "x"); err != nil {
		t.Fatal(err)
	}
	if got, _ := Get(d, mustPath(t, ".metadata.labels.app")); got != "x" {
		t.Errorf("label = %v, want x", got)
	}

	// set a field inside a field-selected element
	if err := Set(d, mustPath(t, ".spec.containers[name=ctl].image"), "img:1"); err != nil {
		t.Fatal(err)
	}
	if got, _ := Get(d, mustPath(t, ".spec.containers[name=ctl].image")); got != "img:1" {
		t.Errorf("image = %v, want img:1", got)
	}
}

func TestSetCannotCreateListElement(t *testing.T) {
	d := sampleDoc()
	if err := Set(d, mustPath(t, ".spec.containers[name=zzz].image"), "x"); err == nil {
		t.Error("expected error creating a missing field-selected element")
	}
}

func TestRemove(t *testing.T) {
	d := sampleDoc()

	if err := Remove(d, mustPath(t, ".spec.replicas")); err != nil {
		t.Fatal(err)
	}
	if _, ok := Get(d, mustPath(t, ".spec.replicas")); ok {
		t.Error("replicas still present after remove")
	}

	// removing a list element shortens the slice and writes it back
	if err := Remove(d, mustPath(t, ".spec.containers[name=side]")); err != nil {
		t.Fatal(err)
	}
	list, _ := Get(d, mustPath(t, ".spec.containers"))
	if l, _ := list.([]any); len(l) != 1 {
		t.Errorf("containers len = %d, want 1", len(l))
	}
	if _, ok := Get(d, mustPath(t, ".spec.containers[name=side]")); ok {
		t.Error("side still present after remove")
	}
}
