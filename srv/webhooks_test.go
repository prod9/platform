package srv

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/fxtest"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

const testWebhookSecret = "whsec"

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
	require.Equal(t, &CreateBuild{
		Owner:    "prod9",
		Repo:     "app",
		CloneURL: "https://github.com/prod9/app.git",
		Tag:      "v1.2.3",
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
	req := httptest.NewRequest("POST", "/api/webhooks/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", event)
	if signature != "" {
		req.Header.Set("X-Hub-Signature-256", signature)
	}
	return req
}

func webhookRouter(t *testing.T) chi.Router {
	router, err := Router(fxtest.Configure())
	require.NoError(t, err)
	return router
}

func TestWebhookWithoutGitHubApp(t *testing.T) {
	stubGitHubApp(t, nil, ErrNoGitHubApp)
	router := webhookRouter(t)

	resp := httptest.NewRecorder()
	body := `{"zen":"ok"}`
	router.ServeHTTP(resp, webhookRequest("ping", body, signBody(testWebhookSecret, body)))

	require.Equal(t, http.StatusServiceUnavailable, resp.Code)
}

func TestWebhookRejectsMissingSignature(t *testing.T) {
	stubGitHubApp(t, &GitHubApp{WebhookSecret: testWebhookSecret}, nil)
	router := webhookRouter(t)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, webhookRequest("ping", `{"zen":"ok"}`, ""))

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestWebhookRejectsBadSignature(t *testing.T) {
	stubGitHubApp(t, &GitHubApp{WebhookSecret: testWebhookSecret}, nil)
	router := webhookRouter(t)

	resp := httptest.NewRecorder()
	body := `{"zen":"ok"}`
	router.ServeHTTP(resp, webhookRequest("ping", body, signBody("wrong-secret", body)))

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestWebhookPingIsNoOp(t *testing.T) {
	stubGitHubApp(t, &GitHubApp{WebhookSecret: testWebhookSecret}, nil)
	router := webhookRouter(t)

	resp := httptest.NewRecorder()
	body := `{"zen":"Design for failure."}`
	router.ServeHTTP(resp, webhookRequest("ping", body, signBody(testWebhookSecret, body)))

	require.Equal(t, http.StatusOK, resp.Code)
}

func TestWebhookMalformedPushBody(t *testing.T) {
	stubGitHubApp(t, &GitHubApp{WebhookSecret: testWebhookSecret}, nil)
	router := webhookRouter(t)

	resp := httptest.NewRecorder()
	body := `{"ref": "refs/tags/v1"` // truncated JSON, correctly signed
	router.ServeHTTP(resp, webhookRequest("push", body, signBody(testWebhookSecret, body)))

	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestWebhookBranchPushIsNoOp(t *testing.T) {
	stubGitHubApp(t, &GitHubApp{WebhookSecret: testWebhookSecret}, nil)
	router := webhookRouter(t)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, webhookRequest("push", branchPushBody, signBody(testWebhookSecret, branchPushBody)))

	require.Equal(t, http.StatusOK, resp.Code)
}

func TestWebhookTagPushCreatesBuild(t *testing.T) {
	ctx := setupDB(t)
	require.NoError(t, (&SaveGitHubApp{WebhookSecret: testWebhookSecret}).Execute(ctx, nil))

	router := webhookRouter(t)
	resp := httptest.NewRecorder()
	req := webhookRequest("push", tagPushBody, signBody(testWebhookSecret, tagPushBody))
	router.ServeHTTP(resp, req.WithContext(ctx))

	require.Equal(t, http.StatusAccepted, resp.Code)

	var build struct {
		Owner    string `db:"owner"`
		Repo     string `db:"repo"`
		CloneURL string `db:"clone_url"`
		Tag      string `db:"tag"`
		SHA      string `db:"sha"`
		Status   string `db:"status"`
		Error    string `db:"error"`
		Image    string `db:"image"`
		Digest   string `db:"digest"`
	}
	require.NoError(t, data.Get(ctx, &build, `
		SELECT owner, repo, clone_url, tag, sha, status, error, image, digest
		FROM builds`))
	require.Equal(t, "prod9", build.Owner)
	require.Equal(t, "app", build.Repo)
	require.Equal(t, "https://github.com/prod9/app.git", build.CloneURL)
	require.Equal(t, "v1.2.3", build.Tag)
	require.Equal(t, "abc123", build.SHA)
	require.Equal(t, "queued", build.Status)
	require.Equal(t, "", build.Error)
	require.Equal(t, "", build.Image)
	require.Equal(t, "", build.Digest)
}

func TestWebhookBranchPushCreatesNoBuild(t *testing.T) {
	ctx := setupDB(t)
	require.NoError(t, (&SaveGitHubApp{WebhookSecret: testWebhookSecret}).Execute(ctx, nil))

	router := webhookRouter(t)
	resp := httptest.NewRecorder()
	req := webhookRequest("push", branchPushBody, signBody(testWebhookSecret, branchPushBody))
	router.ServeHTTP(resp, req.WithContext(ctx))

	require.Equal(t, http.StatusOK, resp.Code)

	var count int
	require.NoError(t, data.Get(ctx, &count, `SELECT count(*) FROM builds`))
	require.Zero(t, count)
}
