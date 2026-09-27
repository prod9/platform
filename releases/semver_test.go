package releases

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Stable bumps and prerelease finalization follow docs/spec/releases.md.
func TestSemverNextName(t *testing.T) {
	cases := []struct {
		previous string
		patch    string
		minor    string
		major    string
	}{
		{"", "v0.1.0", "v0.1.0", "v0.1.0"},
		{"v1.2.3", "v1.2.4", "v1.3.0", "v2.0.0"},
		{"v1.2.3-alpha.1", "v1.2.3", "v1.3.0", "v2.0.0"},
		{"v1.3.0-beta.2", "v1.3.0", "v1.4.0", "v2.0.0"},
		{"v2.0.0-rc.1", "v2.0.0", "v2.1.0", "v3.0.0"},
		{"v1.2.3-alpha.preview.10", "v1.2.3", "v1.3.0", "v2.0.0"},
		{"v1.2.3-alpha.1+incompatible", "v1.2.3", "v1.3.0", "v2.0.0"},
	}
	for _, c := range cases {
		for _, bump := range []Bump{"", BumpAny, BumpPatch, BumpMinor, BumpMajor} {
			t.Run(c.previous+"/"+string(bump), func(t *testing.T) {
				want := c.patch
				switch bump {
				case BumpMinor:
					want = c.minor
				case BumpMajor:
					want = c.major
				}

				got, err := (Semver{}).NextName(c.previous, bump)
				require.NoError(t, err)
				require.Equal(t, want, got)
			})
		}
	}
}
