package builds

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/fxtest"
	"fx.prodigy9.co/httpserver/middlewares"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/auth"
)

func apiRouter(t *testing.T) chi.Router {
	cfg := fxtest.Configure()
	router := chi.NewRouter()
	router.Use(middlewares.Configure(cfg))
	require.NoError(t, APICtr{}.Mount(cfg, router))
	return router
}

// startTestSession seeds a user with a live session, returning the raw session token
// the client-side cookie would carry.
func startTestSession(t *testing.T, ctx context.Context) string {
	var userID int64
	require.NoError(t, data.Get(ctx, &userID,
		`INSERT INTO users (name) VALUES ('octocat') RETURNING id`))

	token := "test-session-token"
	create := &auth.CreateSession{UserID: userID, Token: token, ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, create.Execute(ctx, nil))
	return token
}

func TestListWithoutCookie(t *testing.T) {
	router := apiRouter(t)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/api/builds", nil))

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestListNewestFirst(t *testing.T) {
	ctx := setupDB(t)
	older := queueTestBuild(t, ctx, "app")
	newer := queueTestBuild(t, ctx, "later-app")
	token := startTestSession(t, ctx)
	router := apiRouter(t)

	req := httptest.NewRequest("GET", "/api/builds", nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "platform_session", Value: token})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

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
