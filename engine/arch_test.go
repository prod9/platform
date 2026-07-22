package engine

import (
	"testing"

	fxconfig "fx.prodigy9.co/config"
	r "github.com/stretchr/testify/require"
	"platform.prodigy9.co/conf"
)

// TestBuildArch pins the arch rule: the question is not where you stand but whether the
// image outlives the box that built it. A CI build's output is pushed, so it takes the
// publish arch even though the verb is a plain build.
func TestBuildArch(t *testing.T) {
	cfg := &conf.Model{LocalArch: "auto", PublishArch: "amd64"}
	eng := New(fxconfig.Configure())

	t.Setenv("CI", "")
	r.Equal(t, "auto", eng.buildArch(cfg))

	t.Setenv("CI", "true")
	r.Equal(t, "amd64", eng.buildArch(cfg))
}
