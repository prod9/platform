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
	require.NoError(t, BuildCtr{}.Mount(cfg, router))
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

	body := decodeBuilds(t, resp)
	require.Len(t, body, 2)
	require.Equal(t, newer.ID, body[0].ID)
	require.Equal(t, "later-app", body[0].Repo)
	require.Equal(t, older.ID, body[1].ID)
	require.Equal(t, "app", body[1].Repo)
	require.Equal(t, "prod9", body[0].Owner)
	require.Equal(t, "https://github.com/prod9/later-app.git", body[0].CloneURL)
	require.Equal(t, "github-push", body[0].Trigger)
	require.Equal(t, "refs/tags/v1.2.3", body[0].Ref)
	require.Equal(t, "abc123", body[0].SHA)
}

// A build the engine has said nothing about is queued, and one whose units have all
// finished carries the outcome its stream folds down to — no status is ever stored.
func TestListFoldsEachBuildsEvents(t *testing.T) {
	ctx := setupDB(t)
	queued := queueTestBuild(t, ctx, "app")
	published := queueTestBuild(t, ctx, "later-app")
	appendTestEvents(t, ctx, published.ID,
		&AppendEvent{Kind: EventStepStarted, Unit: "api", At: at(1)},
		&AppendEvent{Kind: EventPublished, Unit: "api", At: at(2),
			Image: "ghcr.io/prod9/later-app:v1.2.3", Hash: "sha256:abc"},
		&AppendEvent{Kind: EventRunDone, Unit: "api", At: at(3)})
	token := startTestSession(t, ctx)
	router := apiRouter(t)

	req := httptest.NewRequest("GET", "/api/builds", nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "platform_session", Value: token})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	body := decodeBuilds(t, resp)
	require.Len(t, body, 2)
	require.Equal(t, published.ID, body[0].ID)
	require.Equal(t, "succeeded", body[0].Status)
	require.Equal(t, "ghcr.io/prod9/later-app:v1.2.3", body[0].Image)
	require.Equal(t, "sha256:abc", body[0].Hash)
	require.Equal(t, queued.ID, body[1].ID)
	require.Equal(t, "queued", body[1].Status)
	require.Empty(t, body[1].Image)
}

// listedBuild mirrors the handler's wire shape; the fragment writes its own wire structs
// by hand (spec §No api/ contract layer), so the test asserts against a hand-written one.
type listedBuild struct {
	ID       int64  `json:"id"`
	Trigger  string `json:"trigger"`
	RetryOf  int64  `json:"retry_of"`
	UserID   int64  `json:"user_id"`
	Owner    string `json:"owner"`
	Repo     string `json:"repo"`
	CloneURL string `json:"clone_url"`
	Ref      string `json:"ref"`
	SHA      string `json:"sha"`
	Status   string `json:"status"`
	Image    string `json:"image"`
	Hash     string `json:"hash"`
	Error    string `json:"error"`
}

func decodeBuilds(t *testing.T, resp *httptest.ResponseRecorder) []listedBuild {
	body := []listedBuild{}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	return body
}

func appendTestEvents(t *testing.T, ctx context.Context, buildID int64, records ...*AppendEvent) {
	for _, record := range records {
		record.BuildID = buildID
		require.NoError(t, record.Execute(ctx, nil))
	}
}

// eventsFor reads one build's stream through the same reader the handler uses.
func eventsFor(t *testing.T, ctx context.Context, buildID int64) []*BuildEvent {
	streams, err := streamsFor(ctx, listLimit)
	require.NoError(t, err)
	return streams[buildID]
}
