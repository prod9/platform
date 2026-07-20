// Package srvtest is the shared test scaffolding for srv fragments: postgres
// availability gating and per-test database setup. It imports no fragment — each
// fragment's tests pass in the migration sources they need, so srvtest stays usable
// from every fragment without import cycles.
package srvtest

import (
	"context"
	"testing"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/dbname"
	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/fxtest"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/migrate"
)

// SetupDB connects a fresh test database and applies the given migration sources,
// skipping the test when postgres is unreachable. SECRET is set so fragments using
// fx secret encryption work without per-test env plumbing.
func SetupDB(t *testing.T, sources ...migrator.Source) context.Context {
	SkipWithoutPostgres(t)
	t.Setenv("SECRET", "the cake is a lie")
	ctx := fxtest.ConnectTestDatabase(t)

	m := migrator.New(data.FromContext(ctx), migrate.Merged(sources...))
	plans, dirty, err := m.Plan(ctx, migrator.IntentMigrate)
	require.NoError(t, err)
	require.False(t, dirty)

	for _, plan := range plans {
		require.NoError(t, m.Apply(ctx, plan))
	}
	return ctx
}

// SkipWithoutPostgres skips the test unless DATABASE_URL points at a reachable
// postgres.
func SkipWithoutPostgres(t *testing.T) {
	url := config.Get(fxtest.Configure(), data.DatabaseURLConfig)
	if url == "" {
		t.Skip("DATABASE_URL unset; skipping postgres-backed test")
	}

	adminURL, err := dbname.Set(url, "postgres")
	if err != nil {
		t.Skipf("unusable DATABASE_URL: %s", err)
	}

	db, err := sqlx.Connect("pgx", adminURL)
	if err != nil {
		t.Skipf("postgres unreachable: %s", err)
	}
	db.Close()
}
