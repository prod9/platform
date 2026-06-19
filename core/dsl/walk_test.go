package dsl

import (
	"reflect"
	"testing"
)

func sampleDoc() Doc {
	return Doc{
		"kind": "Deployment",
		"spec": Doc{
			"replicas": 3,
			"containers": []any{
				Doc{"name": "ctl", "args": []any{"--a"}},
				Doc{"name": "side"},
			},
		},
	}
}

func mustPath(t *testing.T, s string) Path {
	t.Helper()
	p, err := pathFromString(s)
	if err != nil {
		t.Fatalf("pathFromString(%q): %v", s, err)
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
		{".spec.containers[1].name", "side"},
		{".spec.containers[0].args[0]", "--a"},
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
	for _, p := range []string{".nope", ".spec.nope", ".spec.containers[5]"} {
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

	// set a field inside a list element addressed by index
	if err := Set(d, mustPath(t, ".spec.containers[0].image"), "img:1"); err != nil {
		t.Fatal(err)
	}
	if got, _ := Get(d, mustPath(t, ".spec.containers[0].image")); got != "img:1" {
		t.Errorf("image = %v, want img:1", got)
	}
}

// TestSetAutoVivifiesNestedListAndMaps builds the NGF firewall-annotation patch
// shape from an empty doc: deep scalar Sets create the intermediate maps and the
// [0] list slot, and a quoted key carries the dotted/slashed annotation name.
func TestSetAutoVivifiesNestedListAndMaps(t *testing.T) {
	d := Doc{}
	const p = `.spec.kubernetes.service.patches[0].value.metadata.annotations."service.beta.kubernetes.io/linode-loadbalancer-firewall-id"`

	if err := Set(d, mustPath(t, ".spec.kubernetes.service.patches[0].type"), "StrategicMerge"); err != nil {
		t.Fatal(err)
	}
	if err := Set(d, mustPath(t, p), "11222746"); err != nil {
		t.Fatal(err)
	}

	if got, _ := Get(d, mustPath(t, p)); got != "11222746" {
		t.Errorf("annotation = %v, want 11222746", got)
	}
	if got, _ := Get(d, mustPath(t, ".spec.kubernetes.service.patches[0].type")); got != "StrategicMerge" {
		t.Errorf("patch type = %v, want StrategicMerge", got)
	}
	patches, _ := Get(d, mustPath(t, ".spec.kubernetes.service.patches"))
	if list, ok := patches.([]any); !ok || len(list) != 1 {
		t.Errorf("patches = %#v, want a 1-element list", patches)
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
	if err := Remove(d, mustPath(t, ".spec.containers[1]")); err != nil {
		t.Fatal(err)
	}
	list, _ := Get(d, mustPath(t, ".spec.containers"))
	if l, _ := list.([]any); len(l) != 1 {
		t.Errorf("containers len = %d, want 1", len(l))
	}
	if got, _ := Get(d, mustPath(t, ".spec.containers[0].name")); got != "ctl" {
		t.Errorf("remaining container = %v, want ctl", got)
	}
}
