package engine

import (
	"testing"

	r "github.com/stretchr/testify/require"
	"platform.prodigy9.co/conf"
)

// TestBuildArch pins the arch rule: the question is not where you stand but whether the
// image outlives the box that built it. A CI build's output is pushed, so it takes the
// publish arch even though the verb is a plain build.
func TestBuildArch(t *testing.T) {
	cfg := &conf.Model{LocalArch: "auto", PublishArch: "amd64"}
	sess := NewSession(rosterCtx())

	t.Setenv("CI", "")
	r.Equal(t, "auto", sess.buildArch(cfg))

	t.Setenv("CI", "true")
	r.Equal(t, "amd64", sess.buildArch(cfg))
}
