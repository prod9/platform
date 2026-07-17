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

// TestSortReleaseNamesDatestampCounters pins counter ordering: semver reads
// v20260717-1 as a *prerelease* of v20260717 and sorts it below the bare tag, so
// LatestName returned the bare tag and NextName re-yielded an existing counter on the
// next same-day release. Datestamp refs compare by date then counter.
func TestSortReleaseNamesDatestampCounters(t *testing.T) {
	names := []string{"v0.9.10", "v20260710", "v20260717", "v20260717-2", "v20260717-1"}
	sortReleaseNames(names)
	r.Equal(t,
		[]string{"v20260717-2", "v20260717-1", "v20260717", "v20260710", "v0.9.10"},
		names)
}
