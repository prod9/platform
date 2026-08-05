package install

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/fxtest"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/migrate"
	"platform.prodigy9.co/srv/srvtest"
)

func TestComplete(t *testing.T) {
	require.True(t, Complete([]Entry{{State: FullyReadyState}, {State: FullyReadyState}}))
	require.False(t, Complete([]Entry{{State: FullyReadyState}, {State: NotStartedState}}))
	require.False(t, Complete([]Entry{{State: PartiallyReadyState}}))
	require.False(t, Complete([]Entry{{State: UnknownState}}))
}

// With the schema migrated but no credentials entered and no org bound, the ordered
// state reports db/migrations ready and both settings-backed entries not started —
// the wizard's remaining steps (docs/spec/installation.md, the state surface).
func TestGetStateMigratedButNotInstalled(t *testing.T) {
	ctx := srvtest.SetupDB(t, Source)
	db := data.FromContext(ctx)

	entries := GetState(ctx, db, migrate.Merged(Source))

	require.Equal(t, []Entry{
		{Name: "db-reachable", State: FullyReadyState},
		{Name: "migrations", State: FullyReadyState},
		{Name: "app-created", State: NotStartedState},
		{Name: "app-credentials", State: NotStartedState},
		{Name: "app-installed", State: NotStartedState},
	}, entries)
	require.False(t, Complete(entries))
}

// The creation step turns ready on its own trio — the generated keys are the next
// step's concern, so app-credentials stays not started
// (docs/spec/installation.md, the state surface).
func TestCreatedStepReadyBeforeCredentials(t *testing.T) {
	ctx := srvtest.SetupDB(t, Source)
	db := data.FromContext(ctx)

	err := github.SaveAppCreation(ctx, &github.AppCreation{
		AppID:         42,
		ClientID:      "Iv1.abc",
		WebhookSecret: "whsec",
	})
	require.NoError(t, err)

	entries := GetState(ctx, db, migrate.Merged(Source))

	require.Equal(t, FullyReadyState, entries[2].State)
	require.Equal(t, NotStartedState, entries[3].State)
}

// On a fresh database nothing exists yet: every step below db-reachable is not
// started. The settings-backed checks must reach this verdict without ever sending
// a query that can fail to parse — the schema probe, not a caught error, is what
// detects the absent table (docs/spec/installation.md, install-safe checks).
func TestGetStateFreshDBReportsNotStarted(t *testing.T) {
	srvtest.SkipWithoutPostgres(t)
	ctx := fxtest.ConnectTestDatabase(t)
	db := data.FromContext(ctx)

	entries := GetState(ctx, db, migrate.Merged(Source))

	require.Equal(t, []Entry{
		{Name: "db-reachable", State: FullyReadyState},
		{Name: "migrations", State: NotStartedState},
		{Name: "app-created", State: NotStartedState},
		{Name: "app-credentials", State: NotStartedState},
		{Name: "app-installed", State: NotStartedState},
	}, entries)
}

// With only part of the merged set applied, migrations report partially ready —
// the one step that can be (docs/spec/installation.md, the state surface).
func TestGetStatePartialMigrationsIsPartiallyReady(t *testing.T) {
	ctx := srvtest.SetupDB(t, migrate.JobsTable)
	db := data.FromContext(ctx)

	entries := GetState(ctx, db, migrate.Merged(migrate.JobsTable, Source))

	require.Equal(t, PartiallyReadyState, entries[1].State)
}

// Without a database nothing downstream is determinable: db-reachable is the
// operator's to fix, and every later check is unknown rather than a verdict.
func TestGetStateNilDB(t *testing.T) {
	ctx := config.NewContext(context.Background(), fxtest.Configure())
	entries := GetState(ctx, nil, migrate.Merged(Source))

	require.Equal(t, InterventionRequiredState, entries[0].State)
	for _, entry := range entries[1:] {
		require.Equal(t, UnknownState, entry.State)
		require.NotEmpty(t, entry.Message)
	}
	require.False(t, Complete(entries))
}

// Credentials saved but the App under-scoped: the check reads GET /app and reports
// partially ready, naming the gap (docs/spec/installation.md, the credentials check).
func TestCredentialsStepFlagsUnderscopedApp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"permissions":{"contents":"read","metadata":"read","members":"read"}}`)
	}))
	t.Cleanup(server.Close)
	t.Setenv("GITHUB_API_URL", server.URL)

	ctx := srvtest.SetupDB(t, Source)
	srvtest.StubApp(t, srvtest.TestApp(t), nil)
	db := data.FromContext(ctx)

	entries := GetState(ctx, db, migrate.Merged(Source))

	require.Equal(t, PartiallyReadyState, entries[3].State)
	require.Contains(t, entries[3].Message, "contents: write")
}

// A fully scoped App reads fully ready.
func TestCredentialsStepFullyScopedApp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"permissions":{"contents":"write","metadata":"read","members":"read","organization_hooks":"write"}}`)
	}))
	t.Cleanup(server.Close)
	t.Setenv("GITHUB_API_URL", server.URL)

	ctx := srvtest.SetupDB(t, Source)
	srvtest.StubApp(t, srvtest.TestApp(t), nil)
	db := data.FromContext(ctx)

	entries := GetState(ctx, db, migrate.Merged(Source))

	require.Equal(t, FullyReadyState, entries[3].State)
}
