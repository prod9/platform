package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/fxtest"
	"fx.prodigy9.co/httpserver/middlewares"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

const testServerURL = "https://platform.example.com"

func stubApp(t *testing.T, app *App, err error) {
	orig := LoadApp
	LoadApp = func(ctx context.Context) (*App, error) { return app, err }
	t.Cleanup(func() { LoadApp = orig })
}

func setupRouter(t *testing.T, cfg *config.Source) chi.Router {
	router := chi.NewRouter()
	router.Use(middlewares.Configure(cfg))
	require.NoError(t, SetupCtr{}.Mount(cfg, router))
	return router
}

func TestSetupGitHubRendersManifestForm(t *testing.T) {
	stubApp(t, nil, ErrNoApp)
	cfg := fxtest.Configure()
	config.Set(cfg, ServerURLConfig, testServerURL)
	router := setupRouter(t, cfg)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/setup/github", nil))

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Header().Get("Content-Type"), "text/html")

	body := resp.Body.String()
	require.Contains(t, body, "https://github.com/settings/apps/new")
	require.Contains(t, body, testServerURL+"/api/webhooks/github")
	require.Contains(t, body, testServerURL+"/setup/github/callback")
	require.Contains(t, body, "callback_urls")
	require.Contains(t, body, testServerURL+"/api/auth/github/callback")
	require.Contains(t, body, "registry_package")
}

func TestSetupGitHubAlreadyConfigured(t *testing.T) {
	stubApp(t, &App{Slug: "platform-test"}, nil)
	cfg := fxtest.Configure()
	config.Set(cfg, ServerURLConfig, testServerURL)
	router := setupRouter(t, cfg)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/setup/github", nil))

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), "already configured")
	require.Contains(t, resp.Body.String(), "platform-test")
}

func TestSetupGitHubRequiresServerURL(t *testing.T) {
	stubApp(t, nil, ErrNoApp)
	router := setupRouter(t, fxtest.Configure())

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/setup/github", nil))

	require.Equal(t, http.StatusInternalServerError, resp.Code)
	require.Contains(t, resp.Body.String(), "SERVER_URL")
}

func TestSetupCallbackMissingCode(t *testing.T) {
	router := setupRouter(t, fxtest.Configure())

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/setup/github/callback", nil))

	require.Equal(t, http.StatusBadRequest, resp.Code)
	require.Contains(t, resp.Body.String(), "missing code")
}

func TestSetupCallbackExchangeFailure(t *testing.T) {
	github := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(github.Close)

	cfg := fxtest.Configure()
	config.Set(cfg, APIURLConfig, github.URL)
	router := setupRouter(t, cfg)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/setup/github/callback?code=EXPIRED", nil))

	require.Equal(t, http.StatusBadGateway, resp.Code)
}

func TestSetupCallbackDuplicateApp(t *testing.T) {
	ctx := setupDB(t)
	require.NoError(t, (&SaveApp{AppID: 1, Slug: "existing"}).Execute(ctx, nil))
	github := stubManifestConversion(t)

	cfg := fxtest.Configure()
	config.Set(cfg, APIURLConfig, github.URL)
	router := setupRouter(t, cfg)

	resp := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/setup/github/callback?code=CODE123", nil).WithContext(ctx)
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusConflict, resp.Code)
}

// stubManifestConversion answers the manifest conversion with a full credential set.
func stubManifestConversion(t *testing.T) *httptest.Server {
	github := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		require.Equal(t, "POST", req.Method)
		require.Equal(t, "/app-manifests/CODE123/conversions", req.URL.Path)

		resp.WriteHeader(http.StatusCreated)
		resp.Write([]byte(`{
			"id": 42,
			"slug": "platform-test",
			"pem": "-----BEGIN RSA PRIVATE KEY-----",
			"webhook_secret": "whsec",
			"client_id": "Iv1.abc",
			"client_secret": "csec"
		}`))
	}))
	t.Cleanup(github.Close)
	return github
}

func TestExchangeManifest(t *testing.T) {
	github := stubManifestConversion(t)

	creds, err := exchangeManifest(t.Context(), github.Client(), github.URL, "CODE123")
	require.NoError(t, err)
	require.Equal(t, int64(42), creds.ID)
	require.Equal(t, "platform-test", creds.Slug)
	require.Equal(t, "-----BEGIN RSA PRIVATE KEY-----", creds.PEM)
	require.Equal(t, "whsec", creds.WebhookSecret)
	require.Equal(t, "Iv1.abc", creds.ClientID)
	require.Equal(t, "csec", creds.ClientSecret)
}

func TestExchangeManifestRejectsNon201(t *testing.T) {
	github := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(http.StatusNotFound)
		resp.Write([]byte(`{"message": "Not Found"}`))
	}))
	defer github.Close()

	_, err := exchangeManifest(t.Context(), github.Client(), github.URL, "EXPIRED")
	require.ErrorContains(t, err, "404")
}
