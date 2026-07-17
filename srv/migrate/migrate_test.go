package migrate_test

import (
	"testing"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/fxtest"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/migrate"
	"platform.prodigy9.co/srv/srvtest"
)

func TestStateThenRunConverges(t *testing.T) {
	srvtest.SkipWithoutPostgres(t)
	ctx := fxtest.ConnectTestDatabase(t)
	db := data.FromContext(ctx)

	src := migrator.FromSQL("202607181300_create_widgets",
		"CREATE TABLE widgets (id integer PRIMARY KEY);",
		"DROP TABLE widgets;")

	pending, dirty, err := migrate.State(ctx, db, src)
	require.NoError(t, err)
	require.False(t, dirty)
	require.Equal(t, 1, pending)

	require.NoError(t, migrate.Run(ctx, db, src))

	pending, dirty, err = migrate.State(ctx, db, src)
	require.NoError(t, err)
	require.False(t, dirty)
	require.Equal(t, 0, pending)
}
