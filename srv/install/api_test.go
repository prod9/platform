package install

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fx.prodigy9.co/fxtest"
	"fx.prodigy9.co/httpserver/middlewares"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/srvtest"
)

func TestGetInstallReturnsOrderedEntries(t *testing.T) {
	ctx, router := setupAPI(t)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/api/install", nil).WithContext(ctx))
	require.Equal(t, http.StatusOK, resp.Code)

	var entries []Entry
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &entries))
	require.Len(t, entries, 9)
	for i, name := range []string{"db-reachable", "migrations", "server", "org", "app-created", "app-credentials", "registry-token", "app-installed", "claimed"} {
		require.Equal(t, name, entries[i].Name)
	}
}

// The installer controller has no process-local installation state. Its visibility
// belongs to the request-time durable gate around the fragment
// (docs/spec/installation.md, "Boot composition — the application is permanent").
func TestInstallControllerMountsWithoutProcessLocalGate(t *testing.T) {
	cfg := fxtest.Configure()
	router := chi.NewRouter()
	ctr := InstallCtr{}

	require.NoError(t, ctr.Mount(cfg, router))
}

// The server step has its own ungated action, like every settings-backed step
// (docs/spec/installation.md, "The install settings").
func TestPostServerSavesSetting(t *testing.T) {
	ctx, router := setupAPI(t)

	resp := postJSON(ctx, router, "/api/install/server", `{"public_url": "https://platform.example.com"}`)
	require.Equal(t, http.StatusOK, resp.Code)

	publicURL, err := github.LoadPublicURL(ctx)
	require.NoError(t, err)
	require.Equal(t, "https://platform.example.com", publicURL)
}

func TestPostServerRequired(t *testing.T) {
	ctx, router := setupAPI(t)

	resp := postJSON(ctx, router, "/api/install/server", `{"public_url": ""}`)
	require.Equal(t, http.StatusBadRequest, resp.Code)

	_, err := github.LoadPublicURL(ctx)
	require.ErrorIs(t, err, github.ErrNoPublicURL)
}

// The org step has its own ungated action, like every settings-backed step
// (docs/spec/installation.md, "The install settings").
func TestPostOrgSavesSetting(t *testing.T) {
	ctx, router := setupAPI(t)

	resp := postJSON(ctx, router, "/api/install/org", `{"org": "prod9"}`)
	require.Equal(t, http.StatusOK, resp.Code)

	org, err := github.LoadOrg(ctx)
	require.NoError(t, err)
	require.Equal(t, "prod9", org)
}

func TestPostOrgRequired(t *testing.T) {
	ctx, router := setupAPI(t)

	resp := postJSON(ctx, router, "/api/install/org", `{"org": ""}`)
	require.Equal(t, http.StatusBadRequest, resp.Code)

	_, err := github.LoadOrg(ctx)
	require.ErrorIs(t, err, github.ErrNoOrg)
}

// The two App wizard steps together configure the App: the creation POST writes the
// trio GitHub's form yields, the credentials POST the generated pair — read back
// through the real loader (docs/spec/installation.md, "The install settings").
func TestPostAppThenCredentialsSavesSettings(t *testing.T) {
	ctx, router := setupAPI(t)

	resp := postJSON(ctx, router, "/api/install/app", `{
		"app_id": 42,
		"app_slug": "prodigy9-platform",
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
		"app_slug": "prodigy9-platform",
		"client_id": "Iv1.abc",
		"webhook_secret": "whsec"
	}`)
	require.Equal(t, http.StatusOK, resp.Code)

	creation, err := github.LoadAppCreation(ctx)
	require.NoError(t, err)
	require.Equal(t, &github.AppCreation{
		AppID:         42,
		Slug:          "prodigy9-platform",
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

	resp = postJSON(ctx, router, "/api/install/app", `{
		"app_id": 42,
		"client_id": "Iv1.abc",
		"webhook_secret": "whsec"
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

// Without a database no ungated installer mutation can succeed. The controller reports
// the missing DB before parsing a credential body or executing the no-body migration action.
func TestUngatedPostWithoutDBUnavailable(t *testing.T) {
	cfg := fxtest.Configure()
	router := chi.NewRouter()
	router.Use(middlewares.Configure(cfg))
	ctr := InstallCtr{}
	require.NoError(t, ctr.Mount(cfg, router))

	resp := postJSON(context.Background(), router, "/api/install/credentials", `{"private_key": "x"}`)
	require.Equal(t, http.StatusServiceUnavailable, resp.Code)

	resp = postJSON(context.Background(), router, "/api/install/app", `{"app_id": 1}`)
	require.Equal(t, http.StatusServiceUnavailable, resp.Code)

	resp = postJSON(context.Background(), router, "/api/install/migrations", "")
	require.Equal(t, http.StatusServiceUnavailable, resp.Code)
}

func setupAPI(t *testing.T) (context.Context, chi.Router) {
	ctx := srvtest.SetupDB(t)
	cfg := fxtest.Configure()
	router := chi.NewRouter()
	router.Use(middlewares.Configure(cfg))
	ctr := InstallCtr{}
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
