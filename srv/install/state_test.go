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
	"platform.prodigy9.co/srv/srvtest"
)

// Installed-ness is the durable claim alone, not the live conjunction of wizard
// checks (docs/spec/installation.md, "Boot composition — the application is permanent").
func TestIsInstalledReadsClaimedRecord(t *testing.T) {
	ctx := srvtest.SetupDB(t)
	db := data.FromContext(ctx)

	installed, err := IsInstalled(ctx, db)
	require.NoError(t, err)
	require.False(t, installed)

	claimInstalled(t, ctx)
	installed, err = IsInstalled(ctx, db)
	require.NoError(t, err)
	require.True(t, installed)
}

// The installed predicate probes the settings schema before reading install.* so a
// pre-schema database is a normal uninstalled state, not a failing SQL query
// (docs/spec/installation.md, install-safe checks).
func TestIsInstalledIsSafeBeforeSettingsSchema(t *testing.T) {
	srvtest.SkipWithoutPostgres(t)
	ctx := fxtest.ConnectTestDatabase(t)
	db := data.FromContext(ctx)

	installed, err := IsInstalled(ctx, db)
	require.NoError(t, err)
	require.False(t, installed)
}

// With the schema migrated but no credentials entered and no org bound, the ordered
// state reports db/migrations ready and both settings-backed entries not started —
// the wizard's remaining steps (docs/spec/installation.md, the state surface).
func TestGetStateMigratedButNotInstalled(t *testing.T) {
	ctx := srvtest.SetupDB(t)
	db := data.FromContext(ctx)

	entries := GetState(ctx, db)

	require.Equal(t, []Entry{
		{Name: "db-reachable", State: FullyReadyState},
		{Name: "migrations", State: FullyReadyState},
		{Name: "server", State: NotStartedState},
		{Name: "org", State: NotStartedState},
		{Name: "app-created", State: NotStartedState},
		{Name: "app-credentials", State: NotStartedState},
		{Name: "registry-token", State: NotStartedState},
		{Name: "app-installed", State: NotStartedState},
		{Name: "claimed", State: NotStartedState},
	}, entries)
}

// The creation step turns ready on its own trio — the generated keys are the next
// step's concern, so app-credentials stays not started
// (docs/spec/installation.md, the state surface).
func TestCreatedStepReadyBeforeCredentials(t *testing.T) {
	ctx := srvtest.SetupDB(t)
	db := data.FromContext(ctx)

	err := github.SaveAppCreation(ctx, &github.AppCreation{
		AppID:         42,
		Slug:          "prodigy9-platform",
		ClientID:      "Iv1.abc",
		WebhookSecret: "whsec",
	})
	require.NoError(t, err)

	entries := GetState(ctx, db)

	require.Equal(t, FullyReadyState, entries[4].State)
	require.Equal(t, NotStartedState, entries[5].State)
}

// A creation row saved before the slug existed is incomplete: the step must read
// not-started so the wizard walks the operator back through the form.
func TestCreatedStepWithoutSlugNotStarted(t *testing.T) {
	ctx := srvtest.SetupDB(t)
	db := data.FromContext(ctx)

	err := github.SaveAppCreation(ctx, &github.AppCreation{
		AppID:         42,
		ClientID:      "Iv1.abc",
		WebhookSecret: "whsec",
	})
	require.NoError(t, err)

	entries := GetState(ctx, db)

	require.Equal(t, NotStartedState, entries[4].State)
}

// The org step surfaces its saved slug in values — the non-secret form fields a
// re-opened panel re-displays (docs/spec/installation.md, the state surface).
func TestOrgStepSurfacesSlug(t *testing.T) {
	ctx := srvtest.SetupDB(t)
	db := data.FromContext(ctx)

	require.NoError(t, github.SaveOrg(ctx, "prod9"))

	entries := GetState(ctx, db)

	require.Equal(t, Entry{
		Name:   "org",
		State:  FullyReadyState,
		Values: map[string]string{"org": "prod9"},
	}, entries[3])
}

// The server step surfaces its saved public URL in values
// (docs/spec/installation.md, the state surface).
func TestServerStepSurfacesURL(t *testing.T) {
	ctx := srvtest.SetupDB(t)
	db := data.FromContext(ctx)

	require.NoError(t, github.SavePublicURL(ctx, "https://platform.example.com"))

	entries := GetState(ctx, db)

	require.Equal(t, Entry{
		Name:   "server",
		State:  FullyReadyState,
		Values: map[string]string{"public_url": "https://platform.example.com"},
	}, entries[2])
}

// The creation step surfaces its app id and client id, never the webhook secret —
// a secret's presence is implied by the state, not echoed
// (docs/spec/installation.md, the state surface).
func TestCreatedStepSurfacesNonSecretValues(t *testing.T) {
	ctx := srvtest.SetupDB(t)
	db := data.FromContext(ctx)

	err := github.SaveAppCreation(ctx, &github.AppCreation{
		AppID:         42,
		Slug:          "prodigy9-platform",
		ClientID:      "Iv1.abc",
		WebhookSecret: "whsec",
	})
	require.NoError(t, err)

	entries := GetState(ctx, db)

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

	entries := GetState(ctx, db)

	require.Equal(t, []Entry{
		{Name: "db-reachable", State: FullyReadyState},
		{Name: "migrations", State: NotStartedState},
		{Name: "server", State: NotStartedState},
		{Name: "org", State: NotStartedState},
		{Name: "app-created", State: NotStartedState},
		{Name: "app-credentials", State: NotStartedState},
		{Name: "registry-token", State: NotStartedState},
		{Name: "app-installed", State: NotStartedState},
		{Name: "claimed", State: NotStartedState},
	}, entries)
}

// Without a database nothing downstream is determinable: db-reachable is the
// operator's to fix, and every later check is unknown rather than a verdict.
func TestGetStateNilDB(t *testing.T) {
	ctx := config.NewContext(context.Background(), fxtest.Configure())
	entries := GetState(ctx, nil)

	require.Equal(t, InterventionRequiredState, entries[0].State)
	for _, entry := range entries[1:] {
		require.Equal(t, UnknownState, entry.State)
		require.NotEmpty(t, entry.Message)
	}
}

// Credentials saved but the App under-scoped: the check reads GET /app and reports
// partially ready, naming the gap (docs/spec/installation.md, the credentials check).
func TestCredentialsStepFlagsUnderscopedApp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"permissions":{"contents":"read","metadata":"read","members":"read"}}`)
	}))
	t.Cleanup(server.Close)
	t.Setenv("GITHUB_API_URL", server.URL)

	ctx := srvtest.SetupDB(t)
	srvtest.StubApp(t, srvtest.TestApp(t), nil)
	db := data.FromContext(ctx)

	entries := GetState(ctx, db)

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

	ctx := srvtest.SetupDB(t)
	srvtest.StubApp(t, srvtest.TestApp(t), nil)
	db := data.FromContext(ctx)

	entries := GetState(ctx, db)

	require.Equal(t, FullyReadyState, entries[5].State)
}

// The app-installed check asks GitHub with the App's own credentials: the direct
// org-installation lookup for the bound org is the whole verdict, no session
// involved (docs/spec/installation.md, the state surface).
func TestAppInstalledSeesGitHubInstallation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app" {
			fmt.Fprint(w, `{"permissions":{}}`)
			return
		}
		require.Equal(t, "/orgs/prod9/installation", r.URL.Path)
		fmt.Fprint(w, `{"id":900,"account":{"id":1,"login":"prod9"}}`)
	}))
	t.Cleanup(server.Close)
	t.Setenv("GITHUB_API_URL", server.URL)

	ctx := srvtest.SetupDB(t)
	srvtest.StubApp(t, srvtest.TestApp(t), nil)
	require.NoError(t, github.SaveOrg(ctx, "prod9"))
	db := data.FromContext(ctx)

	entries := GetState(ctx, db)

	require.Equal(t, FullyReadyState, entries[7].State)
	require.Equal(t, NotStartedState, entries[8].State)
}

// GitHub's 404 for the bound org means not installed: not started, install
// still the next move.
func TestAppInstalledIgnoresOtherOrgs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	}))
	t.Cleanup(server.Close)
	t.Setenv("GITHUB_API_URL", server.URL)

	ctx := srvtest.SetupDB(t)
	srvtest.StubApp(t, srvtest.TestApp(t), nil)
	require.NoError(t, github.SaveOrg(ctx, "prod9"))
	db := data.FromContext(ctx)

	entries := GetState(ctx, db)

	require.Equal(t, NotStartedState, entries[7].State)
}

// Saving the ghcr token flips registry-token on its own — it neither needs nor
// affects the App steps (docs/spec/installation.md, the state surface).
func TestRegistryTokenStepReady(t *testing.T) {
	ctx := srvtest.SetupDB(t)
	db := data.FromContext(ctx)

	require.NoError(t, github.SaveRegistryToken(ctx, "ghcr.io", "ghp_token"))

	entries := GetState(ctx, db)

	require.Equal(t, FullyReadyState, entries[6].State)
	require.Equal(t, "registry-token", entries[6].Name)
	require.Equal(t, NotStartedState, entries[5].State)
}
