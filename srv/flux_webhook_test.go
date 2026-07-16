package srv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/fxtest"
	"fx.prodigy9.co/secret"
	"github.com/stretchr/testify/require"
)

// hookCreateRecord captures what the stub GitHub saw on the hook-create call.
type hookCreateRecord struct {
	authorization string
	body          string
}

// stubGitHubHooks serves the full flux-webhook path: the repo lookup (reporting push
// as the caller's permission), installation lookup, token mint, and the hook create
// answered with hookStatus.
func stubGitHubHooks(t *testing.T, hookStatus int, push bool, record *hookCreateRecord) *httptest.Server {
	mux := installationAPIMux(t)
	mux.HandleFunc("GET /repos/prod9/app", func(resp http.ResponseWriter, req *http.Request) {
		resp.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(resp, `{"permissions": {"push": %t, "pull": true}}`, push)
	})
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
	github := stubGitHubHooks(t, http.StatusCreated, true, record)
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
	github := stubGitHubHooks(t, http.StatusUnprocessableEntity, true, &hookCreateRecord{})
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

// stubRepoAccess serves just GET /repos/prod9/app for checkRepoPush unit tests.
func stubRepoAccess(t *testing.T, status int, body string) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		require.Equal(t, "GET", req.Method)
		require.Equal(t, "/repos/prod9/app", req.URL.Path)
		require.Equal(t, "Bearer "+testAccessToken, req.Header.Get("Authorization"))

		resp.Header().Set("Content-Type", "application/json")
		resp.WriteHeader(status)
		resp.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestCheckRepoPush(t *testing.T) {
	github := stubRepoAccess(t, http.StatusOK, `{"permissions": {"push": true, "pull": true}}`)

	err := checkRepoPush(t.Context(), github.Client(), github.URL, testAccessToken, "prod9", "app")
	require.NoError(t, err)
}

func TestCheckRepoPushReadOnly(t *testing.T) {
	github := stubRepoAccess(t, http.StatusOK, `{"permissions": {"push": false, "pull": true}}`)

	err := checkRepoPush(t.Context(), github.Client(), github.URL, testAccessToken, "prod9", "app")
	require.ErrorIs(t, err, errNoRepoPush)
	require.ErrorContains(t, err, "prod9/app")
}

func TestCheckRepoPushInvisibleRepo(t *testing.T) {
	github := stubRepoAccess(t, http.StatusNotFound, `{"message": "Not Found"}`)

	err := checkRepoPush(t.Context(), github.Client(), github.URL, testAccessToken, "prod9", "app")
	require.ErrorIs(t, err, errNoRepoPush)
}

func TestFluxWebhookEndpointWithoutSession(t *testing.T) {
	router, err := Router(fxtest.Configure())
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/repos/prod9/app/flux-webhook", strings.NewReader(`{}`))
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

// seedGitHubIdentity links userID to a github identity carrying an encrypted stored
// user token — the state a real login leaves behind.
func seedGitHubIdentity(t *testing.T, ctx context.Context, userID int64, token string) {
	hidden, err := secret.Hide(config.FromContext(ctx), token)
	require.NoError(t, err)
	metadata, err := json.Marshal(map[string]string{"login": "octocat", "token": hidden})
	require.NoError(t, err)

	require.NoError(t, data.Exec(ctx, `
		INSERT INTO identities (user_id, provider, provider_id, kind, metadata)
		VALUES ($1, 'github', '12345', 'login', $2)`, userID, string(metadata)))
}

func postFluxWebhook(t *testing.T, ctx context.Context, githubURL, sessionToken string) *httptest.ResponseRecorder {
	cfg := fxtest.Configure()
	config.Set(cfg, GitHubAPIURLConfig, githubURL)
	router, err := Router(cfg)
	require.NoError(t, err)

	body := `{"receiver_url": "https://flux.example.com/hook/abc", "secret": "hmacsecret"}`
	req := httptest.NewRequest("POST", "/api/repos/prod9/app/flux-webhook",
		strings.NewReader(body)).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sessionToken})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func TestFluxWebhookEndpointConfiguresHook(t *testing.T) {
	ctx := setupDB(t)
	userID, token := startTestSession(t, ctx, time.Now().Add(time.Hour))
	seedGitHubIdentity(t, ctx, userID, testAccessToken)
	_, keyPEM := testAppKey(t)
	stubGitHubApp(t, &GitHubApp{AppID: testAppID, PrivateKey: keyPEM}, nil)
	record := &hookCreateRecord{}
	github := stubGitHubHooks(t, http.StatusCreated, true, record)

	resp := postFluxWebhook(t, ctx, github.URL, token)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "Bearer "+testInstallToken, record.authorization)
	require.Contains(t, record.body, "registry_package")
	require.Contains(t, record.body, "https://flux.example.com/hook/abc")
}

func TestFluxWebhookEndpointForbidsNonPusher(t *testing.T) {
	ctx := setupDB(t)
	userID, token := startTestSession(t, ctx, time.Now().Add(time.Hour))
	seedGitHubIdentity(t, ctx, userID, testAccessToken)
	_, keyPEM := testAppKey(t)
	stubGitHubApp(t, &GitHubApp{AppID: testAppID, PrivateKey: keyPEM}, nil)
	record := &hookCreateRecord{}
	github := stubGitHubHooks(t, http.StatusCreated, false, record)

	resp := postFluxWebhook(t, ctx, github.URL, token)

	require.Equal(t, http.StatusForbidden, resp.Code)
	require.Empty(t, record.body, "hook must not be created without push access")
}

func TestFluxWebhookEndpointWithoutStoredToken(t *testing.T) {
	ctx := setupDB(t)
	_, token := startTestSession(t, ctx, time.Now().Add(time.Hour)) // no github identity
	_, keyPEM := testAppKey(t)
	stubGitHubApp(t, &GitHubApp{AppID: testAppID, PrivateKey: keyPEM}, nil)
	record := &hookCreateRecord{}
	github := stubGitHubHooks(t, http.StatusCreated, true, record)

	resp := postFluxWebhook(t, ctx, github.URL, token)

	require.Equal(t, http.StatusForbidden, resp.Code)
	require.Contains(t, resp.Body.String(), "log in again")
	require.Empty(t, record.body)
}
