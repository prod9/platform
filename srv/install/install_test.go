package install

import (
	"testing"

	"fx.prodigy9.co/app/settings"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/srvtest"
)

// A corrupt value under an install.* key must name the key it came from — the settings
// rows are hand-inspectable state, and a bare strconv error points at nothing.
func TestLoadBadFieldNamesKey(t *testing.T) {
	ctx := srvtest.SetupDB(t, Source)
	for key, value := range map[string]string{
		keyOrgID:             "not-a-number",
		keyOrgLogin:          "prodigy9",
		keyInstallationID:    "7",
		keyInstalledByUserID: "1",
		keyInstalledByLogin:  "chakrit",
		keyInstalledAt:       "2026-01-01T00:00:00Z",
	} {
		upsert := &settings.Upsert{Key: key, Value: value}
		require.NoError(t, upsert.Execute(ctx, &settings.Settings{}))
	}

	_, err := Load(ctx)
	require.ErrorContains(t, err, "install: bad install.org_id")
}
