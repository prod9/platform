package builds

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/fxtest"
	"fx.prodigy9.co/httpserver/middlewares"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/auth"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/install"
	"platform.prodigy9.co/srv/migrate"
	"platform.prodigy9.co/srv/srvtest"
)

func apiRouter(t *testing.T, cfg *config.Source) chi.Router {
	if cfg == nil {
		cfg = fxtest.Configure()
	}

	router := chi.NewRouter()
	router.Use(middlewares.Configure(cfg))
	require.NoError(t, BuildCtr{}.Mount(cfg, router))
	return router
}

// setupInstalled is an installed server for the trigger/repos endpoints: migrated DB
// with the install.* settings claimed, stubbed App credentials, and a fake GitHub
// serving token mints, the repo lookup, ref resolution, and the installation repo list.
func setupInstalled(t *testing.T) (context.Context, *config.Source) {
	ctx := srvtest.SetupDB(t,
		migrate.JobsTable,
		migrator.FromFS(Migrations),
		migrator.FromFS(auth.Migrations),
		install.Source)
	claim := &install.ClaimInstall{
		InstallationID: 7,
		OrgID:          9, OrgLogin: "prodigy9",
		UserID: 1, UserLogin: "chakrit",
	}
	require.NoError(t, claim.Execute(ctx, nil))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /app/installations/7/access_tokens", func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(201)
		fmt.Fprint(resp, `{"token":"ghs_tok"}`)
	})
	mux.HandleFunc("GET /installation/repositories", func(resp http.ResponseWriter, req *http.Request) {
		require.Equal(t, "Bearer ghs_tok", req.Header.Get("Authorization"))
		fmt.Fprint(resp, `{"total_count":2,"repositories":[
			{"name":"app","full_name":"prodigy9/app","owner":{"login":"prodigy9"}},
			{"name":"api","full_name":"prodigy9/api","owner":{"login":"prodigy9"}}]}`)
	})
	mux.HandleFunc("GET /repos/prodigy9/app", func(resp http.ResponseWriter, req *http.Request) {
		require.Equal(t, "Bearer ghs_tok", req.Header.Get("Authorization"))
		fmt.Fprint(resp, `{"clone_url":"https://github.com/prodigy9/app.git"}`)
	})
	mux.HandleFunc("GET /repos/prodigy9/app/commits/tags/v1.2.3", func(resp http.ResponseWriter, req *http.Request) {
		require.Equal(t, "application/vnd.github.sha", req.Header.Get("Accept"))
		fmt.Fprint(resp, "e4c7a1d9")
	})
	mux.HandleFunc("GET /repos/", func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(404)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	srvtest.StubApp(t, srvtest.TestApp(t), nil)

	cfg := fxtest.Configure()
	config.Set(cfg, github.APIURLConfig, server.URL)
	return ctx, cfg
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

func TestRepoBuildsWithoutCookie(t *testing.T) {
	router := apiRouter(t, nil)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/api/repos/prodigy9/app/builds", nil))

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

// The per-repo feed filters to one repo, newest first, and ?limit=N caps the page —
// the landing page fans out ?limit=3 per visible repo (spec §Operations).
func TestRepoBuildsFiltersAndLimits(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	token := startTestSession(t, ctx)
	router := apiRouter(t, cfg)

	for i, repo := range []string{"app", "app", "api"} {
		create := &Create{
			Owner: "prodigy9", Repo: repo, Ref: "refs/heads/main",
			Trigger: TriggerWebUI, UserID: 1,
			CloneURL: "https://github.com/prodigy9/" + repo + ".git",
			SHA:      fmt.Sprintf("sha%d", i),
		}
		require.NoError(t, create.Execute(ctx, &Build{}))
	}

	req := httptest.NewRequest("GET", "/api/repos/prodigy9/app/builds?limit=1", nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "platform_session", Value: token})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var listing []struct {
		Owner string `json:"owner"`
		Repo  string `json:"repo"`
		SHA   string `json:"sha"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &listing))
	require.Len(t, listing, 1)
	require.Equal(t, "app", listing[0].Repo)
	require.Equal(t, "sha1", listing[0].SHA)
}

func TestTriggerWithoutCookie(t *testing.T) {
	router := apiRouter(t, nil)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("POST", "/api/builds",
		strings.NewReader(`{"owner":"prodigy9","repo":"app","ref":"refs/tags/v1.2.3"}`)))

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestTriggerRecordsResolvedIntent(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	token := startTestSession(t, ctx)
	router := apiRouter(t, cfg)

	req := httptest.NewRequest("POST", "/api/builds",
		strings.NewReader(`{"owner":"prodigy9","repo":"app","ref":"refs/tags/v1.2.3"}`)).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "platform_session", Value: token})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusCreated, resp.Code)

	build := &Build{}
	require.NoError(t, data.Get(ctx, build,
		`SELECT `+buildColumns+` FROM builds ORDER BY id DESC LIMIT 1`))
	require.Equal(t, TriggerWebUI, build.Trigger)
	require.Equal(t, "prodigy9", build.Owner)
	require.Equal(t, "app", build.Repo)
	require.Equal(t, "https://github.com/prodigy9/app.git", build.CloneURL)
	require.Equal(t, "refs/tags/v1.2.3", build.Ref)
	require.Equal(t, "e4c7a1d9", build.SHA)
	require.NotZero(t, build.UserID)

	var created struct {
		ID     int64  `json:"id"`
		Status Status `json:"status"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &created))
	require.Equal(t, build.ID, created.ID)
	require.Equal(t, StatusQueued, created.Status)
}

func TestTriggerUnreachableRepo(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	token := startTestSession(t, ctx)
	router := apiRouter(t, cfg)

	req := httptest.NewRequest("POST", "/api/builds",
		strings.NewReader(`{"owner":"prodigy9","repo":"hidden","ref":"refs/tags/v1.2.3"}`)).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "platform_session", Value: token})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestTriggerUnresolvableRef(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	token := startTestSession(t, ctx)
	router := apiRouter(t, cfg)

	req := httptest.NewRequest("POST", "/api/builds",
		strings.NewReader(`{"owner":"prodigy9","repo":"app","ref":"refs/tags/nope"}`)).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "platform_session", Value: token})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestTriggerMissingFields(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	token := startTestSession(t, ctx)
	router := apiRouter(t, cfg)

	for _, body := range []string{
		`{"repo":"app","ref":"refs/tags/v1.2.3"}`,
		`{"owner":"prodigy9","ref":"refs/tags/v1.2.3"}`,
		`{"owner":"prodigy9","repo":"app"}`,
	} {
		req := httptest.NewRequest("POST", "/api/builds", strings.NewReader(body)).WithContext(ctx)
		req.AddCookie(&http.Cookie{Name: "platform_session", Value: token})
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code, "body=%s", body)
	}
}

func TestListWithoutCookie(t *testing.T) {
	router := apiRouter(t, nil)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/api/builds", nil))

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestListNewestFirst(t *testing.T) {
	ctx := setupDB(t)
	older := queueTestBuild(t, ctx, "app")
	newer := queueTestBuild(t, ctx, "later-app")
	token := startTestSession(t, ctx)
	router := apiRouter(t, nil)

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
	router := apiRouter(t, nil)

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

func TestGetWithoutCookie(t *testing.T) {
	router := apiRouter(t, nil)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/api/builds/1", nil))

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestGetUnknownBuild(t *testing.T) {
	ctx := setupDB(t)
	token := startTestSession(t, ctx)
	router := apiRouter(t, nil)

	req := httptest.NewRequest("GET", "/api/builds/999", nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "platform_session", Value: token})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNotFound, resp.Code)
}

// The detail view is the record plus every attempt folded — steps stay behind the
// sub-resource so the heavy output payload never rides the detail read.
func TestGetFoldsEveryAttempt(t *testing.T) {
	ctx := setupDB(t)
	build := queueTestBuild(t, ctx, "app")
	appendTestEvents(t, ctx, build.ID,
		&AppendEvent{Kind: EventStepStarted, Unit: "api", At: at(1)},
		&AppendEvent{Kind: EventRunDone, Unit: "api", At: at(2), Error: "boom"},
		&AppendEvent{Kind: EventStepStarted, Unit: "api", At: at(3)},
		&AppendEvent{Kind: EventPublished, Unit: "api", At: at(4),
			Image: "ghcr.io/prod9/app:v1.2.3", Hash: "sha256:abc"},
		&AppendEvent{Kind: EventRunDone, Unit: "api", At: at(5)})
	token := startTestSession(t, ctx)
	router := apiRouter(t, nil)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/builds/%d", build.ID), nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "platform_session", Value: token})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var body struct {
		listedBuild
		Attempts []struct {
			Status string `json:"status"`
			Image  string `json:"image"`
			Error  string `json:"error"`
		} `json:"attempts"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Equal(t, build.ID, body.ID)
	require.Equal(t, "app", body.Repo)
	require.Len(t, body.Attempts, 2)
	require.Equal(t, "failed", body.Attempts[0].Status)
	require.Equal(t, "boom", body.Attempts[0].Error)
	require.Equal(t, "succeeded", body.Attempts[1].Status)
	require.Equal(t, "ghcr.io/prod9/app:v1.2.3", body.Attempts[1].Image)
}

func TestStepsWithoutCookie(t *testing.T) {
	router := apiRouter(t, nil)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/api/builds/1/steps", nil))

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestStepsUnknownBuild(t *testing.T) {
	ctx := setupDB(t)
	token := startTestSession(t, ctx)
	router := apiRouter(t, nil)

	req := httptest.NewRequest("GET", "/api/builds/999/steps", nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "platform_session", Value: token})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestStepsListsTheFlatFold(t *testing.T) {
	ctx := setupDB(t)
	build := queueTestBuild(t, ctx, "app")
	appendTestEvents(t, ctx, build.ID,
		&AppendEvent{Kind: EventStepStarted, Unit: "api", Step: "test", At: at(1)},
		&AppendEvent{Kind: EventStepDone, Unit: "api", Step: "test", At: at(2),
			Stdout: "ok\n", Stderr: "warn\n"},
		&AppendEvent{Kind: EventRunDone, Unit: "api", At: at(3)})
	token := startTestSession(t, ctx)
	router := apiRouter(t, nil)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/builds/%d/steps", build.ID), nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "platform_session", Value: token})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var steps []struct {
		Attempt int    `json:"attempt"`
		Unit    string `json:"unit"`
		Step    string `json:"step"`
		Stdout  string `json:"stdout"`
		Stderr  string `json:"stderr"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &steps))
	require.Len(t, steps, 1)
	require.Equal(t, 0, steps[0].Attempt)
	require.Equal(t, "api", steps[0].Unit)
	require.Equal(t, "test", steps[0].Step)
	require.Equal(t, "ok\n", steps[0].Stdout)
	require.Equal(t, "warn\n", steps[0].Stderr)
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
	streams, err := streamsFor(ctx, []*Build{{ID: buildID}})
	require.NoError(t, err)
	return streams[buildID]
}
