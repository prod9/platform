package framework

import (
	"path/filepath"
	"testing"

	r "github.com/stretchr/testify/require"
)

// TestDiscover_uniformDispatch drives every testbed through the single Discover loop and
// asserts each resolves to exactly its framework — Infra included, holding no privileged
// position beyond its list order. Infra never reaches Discover in the Scaffold tests
// (those call Infra{}.Scaffold directly), so this is the only guard that Infra is one
// ordinary entry in knownFrameworks: drop it from the loop and infra-basic misresolves;
// let an IsInfra short-circuit over-match and an app testbed does.
func TestDiscover_uniformDispatch(t *testing.T) {
	cases := []struct {
		testbed string
		want    string
	}{
		{"infra-basic", "platform/infra"},
		{"gobasic", "go/basic"},
		{"gowork", "go/workspace"},
		{"pnpmbasic", "pnpm/basic"},
		{"pnpmstatic", "pnpm/static"},
		{"dockerfile", "dockerfile"},
	}

	for _, c := range cases {
		t.Run(c.testbed, func(t *testing.T) {
			fw, err := Discover(filepath.Join("..", "testbeds", c.testbed))
			r.NoError(t, err)
			r.Equal(t, c.want, fw.Name())
		})
	}
}

// TestDiscover_unrecognizedDir confirms Infra claims nothing by default: an empty dir
// resolves to no framework, not Infra.
func TestDiscover_unrecognizedDir(t *testing.T) {
	_, err := Discover(t.TempDir())
	r.ErrorIs(t, err, ErrNoFramework)
}

// TestFindFramework_roundTripsEveryName resolves each framework by its [modules] name
// through the same registry Discover walks — Infra by name is no different from the rest.
func TestFindFramework_roundTripsEveryName(t *testing.T) {
	for _, fw := range knownFrameworks {
		got, err := FindFramework(fw.Name())
		r.NoError(t, err)
		r.Equal(t, fw.Name(), got.Name())
	}
}
