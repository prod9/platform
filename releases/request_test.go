package releases

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExplicitReleaseNameValidation(t *testing.T) {
	for _, name := range []Name{"v1.2.3", "v1.2.3-alpha.1", "v1.2.3-beta.2", "v1.2.3-rc.1"} {
		t.Run(string(name), func(t *testing.T) {
			require.NoError(t, name.validate(Semver{}))
		})
	}
	for _, name := range []Name{"", "1.2.3", "v1", "v1.2", "v01.2.3", "v1.2.3-alpha.01", "v1.2.3-", "v1.2.3+build", "v1.2.3+incompatible"} {
		t.Run(string(name), func(t *testing.T) {
			require.ErrorIs(t, name.validate(Semver{}), ErrBadVersion)
		})
	}
	for _, strategy := range []Strategy{Datestamp{}, Timestamp{}, Rolling{}} {
		require.ErrorIs(t, Name("v1.2.3-alpha.1").validate(strategy), ErrBadStrategy)
	}
}
