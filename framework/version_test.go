package framework

import (
	"testing"

	r "github.com/stretchr/testify/require"
)

// TestPlatformVersionFromModule pins the nearest-release derivation: an exact release
// passes verbatim; a pseudo-version (what any dev or smoke build stamps) resolves to the
// predecessor tag it encodes, so scaffolded launchers stay deterministic between releases;
// anything else is empty — init treats that as a hard error, like a missing SDK version.
func TestPlatformVersionFromModule(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"v0.9.1", "v0.9.1"},
		{"v1.2.3", "v1.2.3"},
		{"v0.9.2-0.20260716150944-7957812567d8", "v0.9.1"},
		{"v0.9.2-0.20260716150944-7957812567d8+dirty", "v0.9.1"},
		{"v1.0.0-0.20260716150944-7957812567d8", ""}, // patch underflow: no predecessor release
		{"(devel)", ""},
		{"", ""},
		{"v0.9.1-rc1", ""}, // prerelease tags aren't a strategy this repo cuts
	}
	for _, c := range cases {
		r.Equal(t, c.want, platformVersionFromModule(c.in), "input %q", c.in)
	}
}
