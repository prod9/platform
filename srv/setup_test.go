package srv

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/fxtest"
	"github.com/stretchr/testify/require"
)

const testServerURL = "https://platform.example.com"

func stubGitHubApp(t *testing.T, app *GitHubApp, err error) {
	orig := loadGitHubApp
	loadGitHubApp = func(ctx context.Context) (*GitHubApp, error) { return app, err }
	t.Cleanup(func() { loadGitHubApp = orig })
}

func TestSetupGitHubRendersManifestForm(t *testing.T) {
	stubGitHubApp(t, nil, ErrNoGitHubApp)
	cfg := fxtest.Configure()
	config.Set(cfg, ServerURLConfig, testServerURL)
	router, err := Router(cfg)
	require.NoError(t, err)

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
	stubGitHubApp(t, &GitHubApp{Slug: "platform-test"}, nil)
	cfg := fxtest.Configure()
	config.Set(cfg, ServerURLConfig, testServerURL)
	router, err := Router(cfg)
	require.NoError(t, err)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/setup/github", nil))

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), "already configured")
	require.Contains(t, resp.Body.String(), "platform-test")
}

func TestSetupGitHubRequiresServerURL(t *testing.T) {
	stubGitHubApp(t, nil, ErrNoGitHubApp)
	router, err := Router(fxtest.Configure())
	require.NoError(t, err)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/setup/github", nil))

	require.Equal(t, http.StatusInternalServerError, resp.Code)
	require.Contains(t, resp.Body.String(), "SERVER_URL")
}

func TestExchangeManifest(t *testing.T) {
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
	defer github.Close()

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
