package system_test

import (
	"testing"

	"fx.prodigy9.co/app"
	"fx.prodigy9.co/app/settings"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/fxtest"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/srvtest"
	"platform.prodigy9.co/srv/system"
)

func TestStateThenRunConvergesUsingRegisteredMigrations(t *testing.T) {
	srvtest.SkipWithoutPostgres(t)
	t.Chdir(t.TempDir())
	app.RegisterMigrations(settings.App.App())

	ctx := fxtest.ConnectTestDatabase(t)
	db := data.FromContext(ctx)
	applied, pending, dirty, err := system.State(ctx, db)
	require.NoError(t, err)
	require.False(t, dirty)
	require.Equal(t, 0, applied)
	require.Positive(t, pending)

	require.NoError(t, system.Run(ctx, db))

	applied, pending, dirty, err = system.State(ctx, db)
	require.NoError(t, err)
	require.False(t, dirty)
	require.Positive(t, applied)
	require.Equal(t, 0, pending)
}
