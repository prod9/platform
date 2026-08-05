package install

import (
	"testing"

	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/srvtest"
)

func TestSaveRegistryTokenRequiresToken(t *testing.T) {
	require.Error(t, (&SaveRegistryToken{}).Validate())
	require.NoError(t, (&SaveRegistryToken{Token: "ghp_token"}).Validate())
}

// The action writes the ghcr key — the one registry the wizard covers; more
// registries are the punted multi-registry UI (docs/spec/installation.md,
// "The registry token").
func TestSaveRegistryTokenWritesGHCRKey(t *testing.T) {
	ctx := srvtest.SetupDB(t, Source)

	action := &SaveRegistryToken{Token: "ghp_token"}
	require.NoError(t, action.Execute(ctx, nil))

	token, err := github.LoadRegistryToken(ctx, "ghcr.io")
	require.NoError(t, err)
	require.Equal(t, "ghp_token", token)
}
