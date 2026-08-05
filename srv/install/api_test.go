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
	require.Len(t, entries, 5)
	for i, name := range []string{"db-reachable", "migrations", "app-credentials", "app-installed", "flux-setup"} {
		require.Equal(t, name, entries[i].Name)
	}
}

// The wizard's credential step: one ungated POST writes all five github.app_* settings,
// read back through the real loader (docs/spec/installation.md, "The install settings").
func TestPostCredentialsSavesSettings(t *testing.T) {
	ctx, router := setupAPI(t)

	resp := postCredentials(ctx, router, `{
		"app_id": 42,
		"private_key": "-----BEGIN RSA PRIVATE KEY-----",
		"webhook_secret": "whsec",
		"client_id": "Iv1.abc",
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

// Every credential is required — a partial save would leave the app half-configured
// with the state surface reporting done.
func TestPostCredentialsAllRequired(t *testing.T) {
	ctx, router := setupAPI(t)

	resp := postCredentials(ctx, router, `{
		"app_id": 42,
		"private_key": "-----BEGIN RSA PRIVATE KEY-----",
		"webhook_secret": "whsec",
		"client_id": "Iv1.abc"
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

	resp := postCredentials(context.Background(), router, `{"app_id": 0}`)
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

func postCredentials(ctx context.Context, router chi.Router, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/install/credentials",
		strings.NewReader(body)).WithContext(ctx)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}
