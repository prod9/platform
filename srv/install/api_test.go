package install

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/fxtest"
	"fx.prodigy9.co/httpserver/middlewares"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/migrate"
	"platform.prodigy9.co/srv/srvtest"
)

func TestGetInstallReturnsOrderedEntries(t *testing.T) {
	ctx, router := setupAPI(t)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/api/install", nil).WithContext(ctx))
	require.Equal(t, http.StatusOK, resp.Code)

	var entries []Entry
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &entries))
	require.Len(t, entries, 6)
	for i, name := range []string{"db-reachable", "migrations", "app-created", "app-credentials", "registry-token", "app-installed"} {
		require.Equal(t, name, entries[i].Name)
	}
}

// The two App wizard steps together configure the App: the creation POST writes the
// trio GitHub's form yields, the credentials POST the generated pair — read back
// through the real loader (docs/spec/installation.md, "The install settings").
func TestPostAppThenCredentialsSavesSettings(t *testing.T) {
	ctx, router := setupAPI(t)

	resp := postJSON(ctx, router, "/api/install/app", `{
		"app_id": 42,
		"client_id": "Iv1.abc",
		"webhook_secret": "whsec"
	}`)
	require.Equal(t, http.StatusOK, resp.Code)

	resp = postJSON(ctx, router, "/api/install/credentials", `{
		"private_key": "-----BEGIN RSA PRIVATE KEY-----",
		"client_secret": "csec"
	}`)
	require.Equal(t, http.StatusOK, resp.Code)

	app, err := github.LoadApp(ctx)
	require.NoError(t, err)
	require.Equal(t, &github.App{
		AppID:         42,
		PrivateKey:    "-----BEGIN RSA PRIVATE KEY-----",
		WebhookSecret: "whsec",
		ClientID:      "Iv1.abc",
		ClientSecret:  "csec",
	}, app)
}

// Creation alone is half the App: the trio reads back, the full credential set does
// not — the state surface keeps app-credentials as the next step.
func TestPostAppAloneLeavesCredentialsUnset(t *testing.T) {
	ctx, router := setupAPI(t)

	resp := postJSON(ctx, router, "/api/install/app", `{
		"app_id": 42,
		"client_id": "Iv1.abc",
		"webhook_secret": "whsec"
	}`)
	require.Equal(t, http.StatusOK, resp.Code)

	creation, err := github.LoadAppCreation(ctx)
	require.NoError(t, err)
	require.Equal(t, &github.AppCreation{
		AppID:         42,
		ClientID:      "Iv1.abc",
		WebhookSecret: "whsec",
	}, creation)

	_, err = github.LoadApp(ctx)
	require.ErrorIs(t, err, github.ErrNoApp)
}

// Every field of each step is required — a partial save would leave the app
// half-configured with the state surface reporting the step done.
func TestPostAppAllRequired(t *testing.T) {
	ctx, router := setupAPI(t)

	resp := postJSON(ctx, router, "/api/install/app", `{
		"app_id": 42,
		"client_id": "Iv1.abc"
	}`)
	require.Equal(t, http.StatusBadRequest, resp.Code)

	_, err := github.LoadAppCreation(ctx)
	require.ErrorIs(t, err, github.ErrNoApp)
}

func TestPostCredentialsAllRequired(t *testing.T) {
	ctx, router := setupAPI(t)

	resp := postJSON(ctx, router, "/api/install/credentials", `{
		"private_key": "-----BEGIN RSA PRIVATE KEY-----"
	}`)
	require.Equal(t, http.StatusBadRequest, resp.Code)

	_, err := github.LoadApp(ctx)
	require.ErrorIs(t, err, github.ErrNoApp)
}

// Without a database no credential post can succeed, whatever the body says — the
// missing DB is reported before the body is even parsed, matching runMigrations.
func TestPostCredentialsWithoutDBUnavailable(t *testing.T) {
	cfg := fxtest.Configure()
	router := chi.NewRouter()
	router.Use(middlewares.Configure(cfg))
	ctr := StateCtr{DB: nil, Merged: migrate.Merged(Source)}
	require.NoError(t, ctr.Mount(cfg, router))

	resp := postJSON(context.Background(), router, "/api/install/credentials", `{"private_key": "x"}`)
	require.Equal(t, http.StatusServiceUnavailable, resp.Code)

	resp = postJSON(context.Background(), router, "/api/install/app", `{"app_id": 1}`)
	require.Equal(t, http.StatusServiceUnavailable, resp.Code)
}

func setupAPI(t *testing.T) (context.Context, chi.Router) {
	ctx := srvtest.SetupDB(t, Source)
	db := data.FromContext(ctx)

	cfg := fxtest.Configure()
	router := chi.NewRouter()
	router.Use(middlewares.Configure(cfg))
	ctr := StateCtr{DB: db, Merged: migrate.Merged(Source)}
	require.NoError(t, ctr.Mount(cfg, router))
	return ctx, router
}

func postJSON(ctx context.Context, router chi.Router, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path,
		strings.NewReader(body)).WithContext(ctx)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}
