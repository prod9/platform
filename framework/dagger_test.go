package framework

import (
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDaggerVersion(t *testing.T) {
	info := &debug.BuildInfo{
		Deps: []*debug.Module{
			{Path: "github.com/other/dep", Version: "v1.2.3"},
			{Path: "dagger.io/dagger", Version: "v0.21.7"},
		},
	}
	require.Equal(t, "v0.21.7", daggerVersion(info))
}

func TestDaggerVersionHonoursReplace(t *testing.T) {
	info := &debug.BuildInfo{
		Deps: []*debug.Module{
			{
				Path:    "dagger.io/dagger",
				Version: "v0.21.7",
				Replace: &debug.Module{Path: "dagger.io/dagger", Version: "v0.99.0-local"},
			},
		},
	}
	require.Equal(t, "v0.99.0-local", daggerVersion(info))
}

func TestDaggerVersionMissing(t *testing.T) {
	info := &debug.BuildInfo{Deps: []*debug.Module{{Path: "github.com/other/dep", Version: "v1.2.3"}}}
	require.Empty(t, daggerVersion(info))
}
