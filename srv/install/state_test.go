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
	buildengine "platform.prodigy9.co/engine"
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
		{Name: "server", State: NotStartedState},
		{Name: "org", State: NotStartedState},
		{Name: "app-created", State: NotStartedState},
		{Name: "app-credentials", State: NotStartedState},
		{Name: "registry-token", State: NotStartedState},
		{Name: "engine", State: NotStartedState},
		{Name: "app-installed", State: NotStartedState},
		{Name: "claimed", State: NotStartedState},
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
		Slug:          "prodigy9-platform",
		ClientID:      "Iv1.abc",
		WebhookSecret: "whsec",
	})
	require.NoError(t, err)

	entries := GetState(ctx, db, migrate.Merged(Source))

	require.Equal(t, FullyReadyState, entries[4].State)
	require.Equal(t, NotStartedState, entries[5].State)
}

// A creation row saved before the slug existed is incomplete: the step must read
// not-started so the wizard walks the operator back through the form.
func TestCreatedStepWithoutSlugNotStarted(t *testing.T) {
	ctx := srvtest.SetupDB(t, Source)
	db := data.FromContext(ctx)

	err := github.SaveAppCreation(ctx, &github.AppCreation{
		AppID:         42,
		ClientID:      "Iv1.abc",
		WebhookSecret: "whsec",
	})
	require.NoError(t, err)

	entries := GetState(ctx, db, migrate.Merged(Source))

	require.Equal(t, NotStartedState, entries[4].State)
}

// The org step surfaces its saved slug in values — the non-secret form fields a
// re-opened panel re-displays (docs/spec/installation.md, the state surface).
func TestOrgStepSurfacesSlug(t *testing.T) {
	ctx := srvtest.SetupDB(t, Source)
	db := data.FromContext(ctx)

	require.NoError(t, github.SaveOrg(ctx, "prod9"))

	entries := GetState(ctx, db, migrate.Merged(Source))

	require.Equal(t, Entry{
		Name:   "org",
		State:  FullyReadyState,
		Values: map[string]string{"org": "prod9"},
	}, entries[3])
}

// The server step surfaces its saved public URL in values
// (docs/spec/installation.md, the state surface).
func TestServerStepSurfacesURL(t *testing.T) {
	ctx := srvtest.SetupDB(t, Source)
	db := data.FromContext(ctx)

	require.NoError(t, github.SavePublicURL(ctx, "https://platform.example.com"))

	entries := GetState(ctx, db, migrate.Merged(Source))

	require.Equal(t, Entry{
		Name:   "server",
		State:  FullyReadyState,
		Values: map[string]string{"public_url": "https://platform.example.com"},
	}, entries[2])
}

// While engine.hosts is unset the not-started entry's values carry the deployment's
// DAGGER_ENGINE seed — the wizard pre-fills what infra provisioned; the save locks
// it in (docs/spec/installation.md, the engine step).
func TestEngineStepPreFillsFromEnvSeed(t *testing.T) {
	ctx := srvtest.SetupDB(t, Source)
	db := data.FromContext(ctx)

	cfg := fxtest.Configure()
	config.Set(cfg, buildengine.DaggerEngineConfig, "dagger-engine.platform.svc")
	entries := GetState(config.NewContext(ctx, cfg), db, migrate.Merged(Source))

	require.Equal(t, Entry{
		Name:   "engine",
		State:  NotStartedState,
		Values: map[string]string{"hosts": "dagger-engine.platform.svc"},
	}, entries[7])
}

// A saved engine binding wins over the env seed and reads fully ready.
func TestEngineStepSurfacesSavedHosts(t *testing.T) {
	ctx := srvtest.SetupDB(t, Source)
	db := data.FromContext(ctx)

	require.NoError(t, github.SaveEngineHosts(ctx, "locked.platform.svc"))

	cfg := fxtest.Configure()
	config.Set(cfg, buildengine.DaggerEngineConfig, "seed.platform.svc")
	entries := GetState(config.NewContext(ctx, cfg), db, migrate.Merged(Source))

	require.Equal(t, Entry{
		Name:   "engine",
		State:  FullyReadyState,
		Values: map[string]string{"hosts": "locked.platform.svc"},
	}, entries[7])
}

// The creation step surfaces its app id and client id, never the webhook secret —
// a secret's presence is implied by the state, not echoed
// (docs/spec/installation.md, the state surface).
func TestCreatedStepSurfacesNonSecretValues(t *testing.T) {
	ctx := srvtest.SetupDB(t, Source)
	db := data.FromContext(ctx)

	err := github.SaveAppCreation(ctx, &github.AppCreation{
		AppID:         42,
		Slug:          "prodigy9-platform",
		ClientID:      "Iv1.abc",
		WebhookSecret: "whsec",
	})
	require.NoError(t, err)

	entries := GetState(ctx, db, migrate.Merged(Source))

	require.Equal(t, map[string]string{
		"app_id":    "42",
		"app_slug":  "prodigy9-platform",
		"client_id": "Iv1.abc",
	}, entries[4].Values)
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
		{Name: "server", State: NotStartedState},
		{Name: "org", State: NotStartedState},
		{Name: "app-created", State: NotStartedState},
		{Name: "app-credentials", State: NotStartedState},
		{Name: "registry-token", State: NotStartedState},
		{Name: "engine", State: NotStartedState},
		{Name: "app-installed", State: NotStartedState},
		{Name: "claimed", State: NotStartedState},
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

	require.Equal(t, PartiallyReadyState, entries[5].State)
	require.Contains(t, entries[5].Message, "contents: write")
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

	require.Equal(t, FullyReadyState, entries[5].State)
}

// The app-installed check asks GitHub with the App's own credentials: the bound
// org among the App's installations is the whole verdict, no session involved
// (docs/spec/installation.md, the state surface).
func TestAppInstalledSeesGitHubInstallation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"id":900,"account":{"id":1,"login":"prod9"}}]`)
	}))
	t.Cleanup(server.Close)
	t.Setenv("GITHUB_API_URL", server.URL)

	ctx := srvtest.SetupDB(t, Source)
	srvtest.StubApp(t, srvtest.TestApp(t), nil)
	require.NoError(t, github.SaveOrg(ctx, "prod9"))
	db := data.FromContext(ctx)

	entries := GetState(ctx, db, migrate.Merged(Source))

	require.Equal(t, FullyReadyState, entries[8].State)
	require.Equal(t, NotStartedState, entries[9].State)
}

// An installation on some other org is not this server's: not started, install
// still the next move.
func TestAppInstalledIgnoresOtherOrgs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"id":900,"account":{"id":1,"login":"someone-else"}}]`)
	}))
	t.Cleanup(server.Close)
	t.Setenv("GITHUB_API_URL", server.URL)

	ctx := srvtest.SetupDB(t, Source)
	srvtest.StubApp(t, srvtest.TestApp(t), nil)
	require.NoError(t, github.SaveOrg(ctx, "prod9"))
	db := data.FromContext(ctx)

	entries := GetState(ctx, db, migrate.Merged(Source))

	require.Equal(t, NotStartedState, entries[8].State)
}

// Saving the ghcr token flips registry-token on its own — it neither needs nor
// affects the App steps (docs/spec/installation.md, the state surface).
func TestRegistryTokenStepReady(t *testing.T) {
	ctx := srvtest.SetupDB(t, Source)
	db := data.FromContext(ctx)

	require.NoError(t, github.SaveRegistryToken(ctx, "ghcr.io", "ghp_token"))

	entries := GetState(ctx, db, migrate.Merged(Source))

	require.Equal(t, FullyReadyState, entries[6].State)
	require.Equal(t, "registry-token", entries[6].Name)
	require.Equal(t, NotStartedState, entries[5].State)
}
