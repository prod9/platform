package builds

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/fxtest"
	"fx.prodigy9.co/httpserver/middlewares"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/auth"
	"platform.prodigy9.co/srv/github"
)

const testWebhookSecret = "whsec"

func stubApp(t *testing.T, app *github.App, err error) {
	orig := github.LoadApp
	github.LoadApp = func(ctx context.Context) (*github.App, error) { return app, err }
	t.Cleanup(func() { github.LoadApp = orig })
}

func signBody(secret string, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyWebhookSignature(t *testing.T) {
	body := []byte(`{"zen":"Design for failure."}`)
	valid := signBody(testWebhookSecret, string(body))

	require.True(t, verifyWebhookSignature(testWebhookSecret, body, valid))
	require.False(t, verifyWebhookSignature(testWebhookSecret, body, ""))
	require.False(t, verifyWebhookSignature(testWebhookSecret, body, "sha256=deadbeef"))
	require.False(t, verifyWebhookSignature(testWebhookSecret, body, "not-a-signature"))
	require.False(t, verifyWebhookSignature(testWebhookSecret, body, strings.TrimPrefix(valid, "sha256=")))
	require.False(t, verifyWebhookSignature("wrong-secret", body, valid))
	require.False(t, verifyWebhookSignature(testWebhookSecret, []byte("tampered"), valid))
}

func TestBuildForPush(t *testing.T) {
	tagPush := pushEvent{
		Ref:     "refs/tags/v1.2.3",
		Deleted: false,
		After:   "abc123",
		Repository: pushRepository{
			Name:     "app",
			CloneURL: "https://github.com/prod9/app.git",
			Owner:    pushOwner{Login: "prod9"},
		},
	}

	create := buildForPush(tagPush)
	require.NotNil(t, create)
	require.Equal(t, &Create{
		Trigger:  TriggerGitHubPush,
		Owner:    "prod9",
		Repo:     "app",
		CloneURL: "https://github.com/prod9/app.git",
		Ref:      "refs/tags/v1.2.3",
		SHA:      "abc123",
	}, create)

	branchPush := tagPush
	branchPush.Ref = "refs/heads/main"
	require.Nil(t, buildForPush(branchPush))

	deletedTag := tagPush
	deletedTag.Deleted = true
	require.Nil(t, buildForPush(deletedTag))

	nonVersionTag := tagPush
	nonVersionTag.Ref = "refs/tags/release-1"
	require.Nil(t, buildForPush(nonVersionTag))
}

const tagPushBody = `{
	"ref": "refs/tags/v1.2.3",
	"deleted": false,
	"after": "abc123",
	"repository": {
		"name": "app",
		"clone_url": "https://github.com/prod9/app.git",
		"owner": {"login": "prod9"}
	}
}`

const branchPushBody = `{
	"ref": "refs/heads/main",
	"deleted": false,
	"after": "abc123",
	"repository": {
		"name": "app",
		"clone_url": "https://github.com/prod9/app.git",
		"owner": {"login": "prod9"}
	}
}`

func webhookRequest(event, body, signature string) *http.Request {
	req := httptest.NewRequest("POST", "/hooks/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", event)
	if signature != "" {
		req.Header.Set("X-Hub-Signature-256", signature)
	}
	return req
}

func webhookRouter(t *testing.T) chi.Router {
	cfg := fxtest.Configure()
	router := chi.NewRouter()
	router.Use(middlewares.Configure(cfg))
	require.NoError(t, WebhookCtr{}.Mount(cfg, router))
	return router
}

func TestWebhookWithoutGitHubApp(t *testing.T) {
	stubApp(t, nil, github.ErrNoApp)
	router := webhookRouter(t)

	resp := httptest.NewRecorder()
	body := `{"zen":"ok"}`
	router.ServeHTTP(resp, webhookRequest("ping", body, signBody(testWebhookSecret, body)))

	require.Equal(t, http.StatusServiceUnavailable, resp.Code)
}

func TestWebhookRejectsMissingSignature(t *testing.T) {
	stubApp(t, &github.App{WebhookSecret: testWebhookSecret}, nil)
	router := webhookRouter(t)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, webhookRequest("ping", `{"zen":"ok"}`, ""))

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestWebhookRejectsBadSignature(t *testing.T) {
	stubApp(t, &github.App{WebhookSecret: testWebhookSecret}, nil)
	router := webhookRouter(t)

	resp := httptest.NewRecorder()
	body := `{"zen":"ok"}`
	router.ServeHTTP(resp, webhookRequest("ping", body, signBody("wrong-secret", body)))

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestWebhookPingIsNoOp(t *testing.T) {
	stubApp(t, &github.App{WebhookSecret: testWebhookSecret}, nil)
	router := webhookRouter(t)

	resp := httptest.NewRecorder()
	body := `{"zen":"Design for failure."}`
	router.ServeHTTP(resp, webhookRequest("ping", body, signBody(testWebhookSecret, body)))

	require.Equal(t, http.StatusOK, resp.Code)
}

func TestWebhookMalformedPushBody(t *testing.T) {
	stubApp(t, &github.App{WebhookSecret: testWebhookSecret}, nil)
	router := webhookRouter(t)

	resp := httptest.NewRecorder()
	body := `{"ref": "refs/tags/v1"` // truncated JSON, correctly signed
	router.ServeHTTP(resp, webhookRequest("push", body, signBody(testWebhookSecret, body)))

	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestWebhookBranchPushIsNoOp(t *testing.T) {
	stubApp(t, &github.App{WebhookSecret: testWebhookSecret}, nil)
	router := webhookRouter(t)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, webhookRequest("push", branchPushBody, signBody(testWebhookSecret, branchPushBody)))

	require.Equal(t, http.StatusOK, resp.Code)
}

func TestWebhookTagPushCreatesBuild(t *testing.T) {
	ctx := setupDB(t)
	stubApp(t, &github.App{WebhookSecret: testWebhookSecret}, nil)

	router := webhookRouter(t)
	resp := httptest.NewRecorder()
	req := webhookRequest("push", tagPushBody, signBody(testWebhookSecret, tagPushBody))
	router.ServeHTTP(resp, req.WithContext(ctx))

	require.Equal(t, http.StatusAccepted, resp.Code)

	systemUserID, err := auth.SystemUserID(ctx)
	require.NoError(t, err)

	build := &Build{}
	require.NoError(t, data.Get(ctx, build, `SELECT `+buildColumns+` FROM builds`))
	require.Equal(t, TriggerGitHubPush, build.Trigger)
	require.Equal(t, systemUserID, build.UserID)
	require.Zero(t, build.RetryOf)
	require.Equal(t, "prod9", build.Owner)
	require.Equal(t, "app", build.Repo)
	require.Equal(t, "https://github.com/prod9/app.git", build.CloneURL)
	require.Equal(t, "refs/tags/v1.2.3", build.Ref)
	require.Equal(t, "abc123", build.SHA)

	// The record says who asked and what for; how it goes is the event stream's to say.
	require.Empty(t, eventsFor(t, ctx, build.ID))
}

func TestWebhookBranchPushCreatesNoBuild(t *testing.T) {
	ctx := setupDB(t)
	stubApp(t, &github.App{WebhookSecret: testWebhookSecret}, nil)

	router := webhookRouter(t)
	resp := httptest.NewRecorder()
	req := webhookRequest("push", branchPushBody, signBody(testWebhookSecret, branchPushBody))
	router.ServeHTTP(resp, req.WithContext(ctx))

	require.Equal(t, http.StatusOK, resp.Code)

	var count int
	require.NoError(t, data.Get(ctx, &count, `SELECT count(*) FROM builds`))
	require.Zero(t, count)
}
