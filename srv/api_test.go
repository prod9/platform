package srv

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/fxtest"
	"github.com/stretchr/testify/require"
)

// startTestSession seeds a user with a live (or expired) session, returning the user
// id and the raw session token the client-side cookie would carry.
func startTestSession(t *testing.T, ctx context.Context, expiresAt time.Time) (int64, string) {
	var userID int64
	require.NoError(t, data.Get(ctx, &userID,
		`INSERT INTO users (name) VALUES ('octocat') RETURNING id`))

	token := randomToken()
	create := &CreateSession{
		UserID:    userID,
		TokenHash: hashSessionToken(token),
		ExpiresAt: expiresAt,
	}
	require.NoError(t, create.Execute(ctx, nil))
	return userID, token
}

func apiRequest(ctx context.Context, path, token string) *http.Request {
	req := httptest.NewRequest("GET", path, nil).WithContext(ctx)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	}
	return req
}

func TestMeWithoutCookie(t *testing.T) {
	router, err := Router(fxtest.Configure())
	require.NoError(t, err)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/api/me", nil))

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestMeWithSession(t *testing.T) {
	ctx := setupDB(t)
	userID, token := startTestSession(t, ctx, time.Now().Add(time.Hour))
	router, err := Router(fxtest.Configure())
	require.NoError(t, err)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, apiRequest(ctx, "/api/me", token))

	require.Equal(t, http.StatusOK, resp.Code)

	var body struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Equal(t, userID, body.ID)
	require.Equal(t, "octocat", body.Name)
}

func TestMeWithExpiredSession(t *testing.T) {
	ctx := setupDB(t)
	_, token := startTestSession(t, ctx, time.Now().Add(-time.Hour))
	router, err := Router(fxtest.Configure())
	require.NoError(t, err)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, apiRequest(ctx, "/api/me", token))

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestBuildsWithoutCookie(t *testing.T) {
	router, err := Router(fxtest.Configure())
	require.NoError(t, err)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/api/builds", nil))

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestBuildsNewestFirst(t *testing.T) {
	ctx := setupDB(t)
	older := queueTestBuild(t, ctx, "app")
	newer := queueTestBuild(t, ctx, "later-app")
	_, token := startTestSession(t, ctx, time.Now().Add(time.Hour))
	router, err := Router(fxtest.Configure())
	require.NoError(t, err)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, apiRequest(ctx, "/api/builds", token))

	require.Equal(t, http.StatusOK, resp.Code)

	var body []struct {
		ID       int64  `json:"id"`
		Owner    string `json:"owner"`
		Repo     string `json:"repo"`
		CloneURL string `json:"clone_url"`
		Tag      string `json:"tag"`
		SHA      string `json:"sha"`
		Status   string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Len(t, body, 2)
	require.Equal(t, newer.ID, body[0].ID)
	require.Equal(t, "later-app", body[0].Repo)
	require.Equal(t, older.ID, body[1].ID)
	require.Equal(t, "app", body[1].Repo)
	require.Equal(t, "prod9", body[0].Owner)
	require.Equal(t, "https://github.com/prod9/later-app.git", body[0].CloneURL)
	require.Equal(t, "v1.2.3", body[0].Tag)
	require.Equal(t, "abc123", body[0].SHA)
	require.Equal(t, "queued", body[0].Status)
}
