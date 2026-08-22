// Package srvtest is the shared test scaffolding for srv fragments: postgres
// availability gating and per-test database setup. It imports no fragment: test
// packages register their application trees, and SetupDB consumes fx's registered
// migration source without creating fragment import cycles.
package srvtest

import (
	"context"
	"testing"

	"fx.prodigy9.co/app/settings"
	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/dbname"
	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/fxtest"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

// SetupDB connects a fresh test database and applies the migrations registered by the
// composed fx application, skipping when postgres is unreachable. SECRET is set so
// fragments using fx secret encryption work without per-test env plumbing.
func SetupDB(t *testing.T) context.Context {
	SkipWithoutPostgres(t)
	t.Setenv("SECRET", "the cake is a lie")
	t.Chdir(t.TempDir())
	ctx := fxtest.ConnectTestDatabase(t)

	source := migrator.FromAuto(config.FromContext(ctx))
	m := migrator.New(data.FromContext(ctx), source)
	plans, dirty, err := m.Plan(ctx, migrator.IntentMigrate)
	require.NoError(t, err)
	require.False(t, dirty)

	for _, plan := range plans {
		require.NoError(t, m.Apply(ctx, plan))
	}
	return ctx
}

// SeedSettings writes settings required by a test fixture.
func SeedSettings(ctx context.Context, values map[string]string) error {
	return data.Run(ctx, func(scope data.Scope) error {
		for key, value := range values {
			if err := (&settings.Upsert{Key: key, Value: value}).Execute(scope.Context(), &settings.Settings{}); err != nil {
				return err
			}
		}
		return nil
	})
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
	require.NoError(t, db.Close())
}
