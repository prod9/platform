package srv

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/fxtest"
	"github.com/stretchr/testify/require"
)

// hookCreateRecord captures what the stub GitHub saw on the hook-create call.
type hookCreateRecord struct {
	authorization string
	body          string
}

// stubGitHubHooks serves the full ConfigureFluxWebhook path: installation lookup,
// token mint, and the hook create answered with hookStatus.
func stubGitHubHooks(t *testing.T, hookStatus int, record *hookCreateRecord) *httptest.Server {
	mux := installationAPIMux(t)
	mux.HandleFunc("POST /repos/prod9/app/hooks", func(resp http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		record.authorization = req.Header.Get("Authorization")
		record.body = string(body)

		resp.Header().Set("Content-Type", "application/json")
		resp.WriteHeader(hookStatus)
		resp.Write([]byte(`{}`))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func fluxWebhookAction(t *testing.T, apiURL string) (*ConfigureFluxWebhook, context.Context) {
	_, keyPEM := testAppKey(t)
	cfg := fxtest.Configure()
	config.Set(cfg, GitHubAPIURLConfig, apiURL)

	action := &ConfigureFluxWebhook{
		Owner:       "prod9",
		Repo:        "app",
		ReceiverURL: "https://flux.example.com/hook/abc",
		Secret:      "hmacsecret",
		app:         &GitHubApp{AppID: testAppID, PrivateKey: keyPEM},
	}
	return action, config.NewContext(t.Context(), cfg)
}

func TestConfigureFluxWebhook(t *testing.T) {
	record := &hookCreateRecord{}
	github := stubGitHubHooks(t, http.StatusCreated, record)
	action, ctx := fluxWebhookAction(t, github.URL)

	require.NoError(t, action.Execute(ctx, nil))

	require.Equal(t, "Bearer "+testInstallToken, record.authorization)
	require.JSONEq(t, `{
		"name": "web",
		"active": true,
		"events": ["registry_package"],
		"config": {
			"url": "https://flux.example.com/hook/abc",
			"content_type": "json",
			"secret": "hmacsecret"
		}
	}`, record.body)
}

func TestConfigureFluxWebhookDuplicate(t *testing.T) {
	github := stubGitHubHooks(t, http.StatusUnprocessableEntity, &hookCreateRecord{})
	action, ctx := fluxWebhookAction(t, github.URL)

	require.ErrorIs(t, action.Execute(ctx, nil), ErrWebhookExists)
}

func TestConfigureFluxWebhookValidation(t *testing.T) {
	valid, _ := fluxWebhookAction(t, "unused")
	require.NoError(t, valid.Validate())

	insecure := *valid
	insecure.ReceiverURL = "http://flux.example.com/hook/abc"
	require.Error(t, insecure.Validate())

	noReceiver := *valid
	noReceiver.ReceiverURL = ""
	require.Error(t, noReceiver.Validate())

	noSecret := *valid
	noSecret.Secret = ""
	require.Error(t, noSecret.Validate())

	hostileOwner := *valid
	hostileOwner.Owner = "../etc"
	require.Error(t, hostileOwner.Validate())

	noRepo := *valid
	noRepo.Repo = ""
	require.Error(t, noRepo.Validate())
}

func TestFluxWebhookEndpointWithoutSession(t *testing.T) {
	router, err := Router(fxtest.Configure())
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/repos/prod9/app/flux-webhook", strings.NewReader(`{}`))
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestFluxWebhookEndpointConfiguresHook(t *testing.T) {
	ctx := setupDB(t)
	_, token := startTestSession(t, ctx, time.Now().Add(time.Hour))
	_, keyPEM := testAppKey(t)
	stubGitHubApp(t, &GitHubApp{AppID: testAppID, PrivateKey: keyPEM}, nil)
	record := &hookCreateRecord{}
	github := stubGitHubHooks(t, http.StatusCreated, record)

	cfg := fxtest.Configure()
	config.Set(cfg, GitHubAPIURLConfig, github.URL)
	router, err := Router(cfg)
	require.NoError(t, err)

	body := `{"receiver_url": "https://flux.example.com/hook/abc", "secret": "hmacsecret"}`
	req := httptest.NewRequest("POST", "/api/repos/prod9/app/flux-webhook",
		strings.NewReader(body)).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "Bearer "+testInstallToken, record.authorization)
	require.Contains(t, record.body, "registry_package")
	require.Contains(t, record.body, "https://flux.example.com/hook/abc")
}
