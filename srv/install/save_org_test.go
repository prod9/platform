package install

import (
	"testing"

	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/srvtest"
)

func TestSaveOrgRequiresOrg(t *testing.T) {
	require.Error(t, (&SaveOrg{}).Validate())
	require.NoError(t, (&SaveOrg{Org: "prod9"}).Validate())
}

func TestSaveOrgWritesSetting(t *testing.T) {
	ctx := srvtest.SetupDB(t)

	require.NoError(t, (&SaveOrg{Org: "prod9"}).Execute(ctx, nil))

	org, err := github.LoadOrg(ctx)
	require.NoError(t, err)
	require.Equal(t, "prod9", org)
}
