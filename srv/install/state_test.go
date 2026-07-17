package install

import (
	"context"
	"testing"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/fxtest"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/migrate"
	"platform.prodigy9.co/srv/srvtest"
)

func TestComplete(t *testing.T) {
	require.True(t, Complete([]Entry{{Status: StatusDone}, {Status: StatusDone}}))
	require.False(t, Complete([]Entry{{Status: StatusDone}, {Status: StatusPending}}))
	require.False(t, Complete([]Entry{{Status: StatusError}}))
}

// With the schema migrated but no org bound and no app configured, the ordered state
// reports db/migrations done, credentials missing, and the install pending.
func TestGetStateMigratedButNotInstalled(t *testing.T) {
	ctx := srvtest.SetupDB(t, migrator.FromFS(Migrations))
	db := data.FromContext(ctx)

	entries := GetState(ctx, db, migrate.Merged(migrator.FromFS(Migrations)))

	require.Equal(t,
		[]string{"db-reachable", "app-credentials", "app-installed", "migrations"},
		names(entries))
	require.Equal(t, StatusDone, statusOf(entries, "db-reachable"))
	require.Equal(t, StatusError, statusOf(entries, "app-credentials"))
	require.Equal(t, StatusPending, statusOf(entries, "app-installed"))
	require.Equal(t, StatusDone, statusOf(entries, "migrations"))
	require.False(t, Complete(entries))
}

// On a fresh database the installations table is absent; the reader treats the missing
// table as not-installed, and migrations report pending rather than erroring.
func TestGetStateFreshDBReportsPending(t *testing.T) {
	srvtest.SkipWithoutPostgres(t)
	ctx := fxtest.ConnectTestDatabase(t)
	db := data.FromContext(ctx)

	entries := GetState(ctx, db, migrate.Merged(migrator.FromFS(Migrations)))

	require.Equal(t, StatusDone, statusOf(entries, "db-reachable"))
	require.Equal(t, StatusPending, statusOf(entries, "app-installed"))
	require.Equal(t, StatusPending, statusOf(entries, "migrations"))
}

// GetState mirrors an absent database as errors rather than panicking on a nil handle.
func TestGetStateNilDB(t *testing.T) {
	ctx := config.NewContext(context.Background(), fxtest.Configure())
	entries := GetState(ctx, nil, migrate.Merged(migrator.FromFS(Migrations)))

	require.Equal(t, StatusError, statusOf(entries, "db-reachable"))
	require.Equal(t, StatusError, statusOf(entries, "app-installed"))
	require.Equal(t, StatusError, statusOf(entries, "migrations"))
	require.False(t, Complete(entries))
}

func names(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, entry := range entries {
		out[i] = entry.Name
	}
	return out
}

func statusOf(entries []Entry, name string) string {
	for _, entry := range entries {
		if entry.Name == name {
			return entry.Status
		}
	}
	return ""
}
