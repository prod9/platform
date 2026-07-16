package releases

import (
	"testing"

	r "github.com/stretchr/testify/require"
)

// TestSortReleaseNames pins version-aware ordering: lexicographic sort put v0.9.9 above
// v0.9.10, so LatestName returned the wrong latest at any double-digit segment and
// release tried to re-cut an existing tag. Semver compares numerically; names that
// aren't semver (timestamp/datestamp refs) keep byte order among themselves.
func TestSortReleaseNames(t *testing.T) {
	names := []string{"v0.9.9", "v0.9.10", "v0.8.4", "v0.10.0", "v0.9.2"}
	sortReleaseNames(names)
	r.Equal(t, []string{"v0.10.0", "v0.9.10", "v0.9.9", "v0.9.2", "v0.8.4"}, names)
}
