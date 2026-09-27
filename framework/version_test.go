package framework

import (
	"testing"

	r "github.com/stretchr/testify/require"
)

// Launcher pins preserve valid build versions verbatim, including prereleases and
// metadata, as required by docs/spec/scaffolding.md.
func TestPlatformVersionFromModule(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"v0.9.1", "v0.9.1"},
		{"v1.2.3", "v1.2.3"},
		{"v1.2.3-rc.1", "v1.2.3-rc.1"},
		{"v1.2.3+build.7", "v1.2.3+build.7"},
		{"v1.2.3-rc.1+build.7", "v1.2.3-rc.1+build.7"},
		{"", ""},
		{"not-a-version", ""},
		{"v1.02.3", ""},
	}
	for _, c := range cases {
		r.Equal(t, c.want, platformVersionFromModule(c.in), "input %q", c.in)
	}
}
