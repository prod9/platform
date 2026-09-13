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

	"fx.prodigy9.co/app"
	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/fxtest"
	"fx.prodigy9.co/httpserver/middlewares"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/auth"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/install"
	"platform.prodigy9.co/srv/repos"
	"platform.prodigy9.co/srv/srvtest"
)

func init() {
	app.RegisterMigrations(App.App())
	app.RegisterMigrations(auth.App.App())
	app.RegisterMigrations(install.App.App())
	app.RegisterMigrations(repos.App.App())
}

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
// serving token mints, repo lookup, and ref resolution.
func setupInstalled(t *testing.T) (context.Context, *config.Source) {
	return setupInstalledManifest(t, testManifest, http.StatusOK)
}

func setupInstalledManifest(t *testing.T, manifest string, status int) (context.Context, *config.Source) {
	ctx := srvtest.SetupDB(t)
	require.NoError(t, srvtest.SeedSettings(ctx, map[string]string{
		"install.installation_id": "7", "install.org_id": "9", "install.org_login": "prodigy9",
		"install.installed_by_user_id": "1", "install.installed_by_login": "chakrit",
		"install.installed_at": "2026-08-21T00:00:00Z",
	}))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /app/installations/7/access_tokens", func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(201)
		fmt.Fprint(resp, `{"token":"ghs_tok"}`)
	})
	mux.HandleFunc("GET /repos/prodigy9/app", func(resp http.ResponseWriter, req *http.Request) {
		require.Equal(t, "Bearer ghs_tok", req.Header.Get("Authorization"))
		fmt.Fprint(resp, `{"clone_url":"https://github.com/prodigy9/app.git"}`)
	})
	mux.HandleFunc("GET /repos/prodigy9/app/commits/tags/v1.2.3", func(resp http.ResponseWriter, req *http.Request) {
		require.Equal(t, "application/vnd.github.sha", req.Header.Get("Accept"))
		fmt.Fprint(resp, "e4c7a1d9")
	})
	mux.HandleFunc("GET /repos/prodigy9/app/contents/platform.toml", func(resp http.ResponseWriter, req *http.Request) {
		require.Equal(t, "e4c7a1d9", req.URL.Query().Get("ref"))
		resp.WriteHeader(status)
		fmt.Fprint(resp, manifest)
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

// startTestSession seeds a user with a live session and repository snapshot, returning
// the raw session token the client-side cookie carries.
func startTestSession(t *testing.T, ctx context.Context, repositories ...auth.Repository) string {
	var userID int64
	require.NoError(t, data.Get(ctx, &userID,
		`INSERT INTO users (name) VALUES ('octocat') RETURNING id`))

	token := "test-session-token"
	create := &auth.CreateSession{UserID: userID, Token: token, ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, create.Execute(ctx, nil))
	for _, repository := range repositories {
		require.NoError(t, data.Exec(ctx, `
			INSERT INTO session_repositories
				(session_id, github_repository_id, owner, name, permission)
			SELECT id, $1, $2, $3, $4 FROM sessions WHERE user_id = $5`,
			repository.GitHubID, repository.Owner, repository.Name, repository.Permission, userID))
	}
	return token
}

func testRepository(name string, permission auth.Permission) auth.Repository {
	return auth.Repository{GitHubID: int64(len(name)), Owner: "prod9", Name: name, Permission: permission}
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
	token := startTestSession(t, ctx, auth.Repository{GitHubID: 1, Owner: "prodigy9", Name: "app", Permission: auth.PermissionRead})
	router := apiRouter(t, cfg)
	for i, repo := range []string{"app", "app", "api"} {
		registerTestRepo(t, ctx, "prodigy9", repo)
		create := &Create{Owner: "prodigy9", Repo: repo, Ref: "refs/heads/main", Trigger: TriggerWebUI, UserID: 1,
			CloneURL: "https://github.com/prodigy9/" + repo + ".git", SHA: fmt.Sprintf("sha%d", i)}
		prepareCreate(t, ctx, create)
		require.NoError(t, create.Execute(ctx, &Build{}))
	}
	resp := requestBuilds(t, ctx, router, token, "GET", "/api/repos/prodigy9/app/builds?limit=1", "")
	require.Equal(t, http.StatusOK, resp.Code)
	listing := decodeBuilds(t, resp)
	require.Len(t, listing, 1)
	require.Equal(t, "app", listing[0].Repo)
	require.Equal(t, "sha1", listing[0].SHA)
}

func TestRepoBuildsRequireReadPermission(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	token := startTestSession(t, ctx)
	router := apiRouter(t, cfg)

	req := httptest.NewRequest("GET", "/api/repos/prodigy9/app/builds", nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "platform_session", Value: token})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestTriggerWithoutCookie(t *testing.T) {
	router := apiRouter(t, nil)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("POST", "/api/builds",
		strings.NewReader(`{"owner":"prodigy9","repo":"app","ref":"refs/tags/v1.2.3"}`)))

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestTriggerUnregisteredRepository(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	token := startTestSession(t, ctx, auth.Repository{
		GitHubID: 1, Owner: "prodigy9", Name: "app", Permission: auth.PermissionWrite,
	})
	router := apiRouter(t, cfg)

	req := httptest.NewRequest("POST", "/api/builds",
		strings.NewReader(`{"owner":"prodigy9","repo":"app","ref":"refs/tags/v1.2.3"}`)).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "platform_session", Value: token})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNotFound, resp.Code)
	var count int
	require.NoError(t, data.Get(ctx, &count, `SELECT count(*) FROM builds`))
	require.Zero(t, count)
}

func TestTriggerRecordsResolvedIntent(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	registerTestRepo(t, ctx, "prodigy9", "app")
	token := startTestSession(t, ctx, auth.Repository{
		GitHubID: 1, Owner: "prodigy9", Name: "app", Permission: auth.PermissionWrite,
	})
	router := apiRouter(t, cfg)

	req := httptest.NewRequest("POST", "/api/builds",
		strings.NewReader(`{"owner":"prodigy9","repo":"app","ref":"refs/tags/v1.2.3"}`)).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "platform_session", Value: token})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusCreated, resp.Code)

	build := &Build{}
	require.NoError(t, data.Get(ctx, build,
		`SELECT `+buildColumns+` FROM `+buildFrom+` ORDER BY b.id DESC LIMIT 1`))
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

func TestTriggerRequiresWritePermission(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	token := startTestSession(t, ctx, auth.Repository{
		GitHubID: 1, Owner: "prodigy9", Name: "app", Permission: auth.PermissionRead,
	})
	router := apiRouter(t, cfg)

	req := httptest.NewRequest("POST", "/api/builds",
		strings.NewReader(`{"owner":"prodigy9","repo":"app","ref":"refs/tags/v1.2.3"}`)).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "platform_session", Value: token})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestTriggerUnresolvableRef(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	registerTestRepo(t, ctx, "prodigy9", "app")
	token := startTestSession(t, ctx, auth.Repository{
		GitHubID: 1, Owner: "prodigy9", Name: "app", Permission: auth.PermissionWrite,
	})
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
	token := startTestSession(t, ctx,
		testRepository("app", auth.PermissionRead),
		testRepository("later-app", auth.PermissionRead))
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
	modules := modulesFor(t, ctx, published.ID)
	for _, module := range modules {
		claimTestModule(t, ctx, module)
		appendTestEvents(t, ctx, module.ID, &AppendEvent{Kind: EventRunDone, At: at(3), Image: "ghcr.io/prod9/later-app:v1.2.3", Hash: "sha256:abc"})
	}
	token := startTestSession(t, ctx, testRepository("app", auth.PermissionRead), testRepository("later-app", auth.PermissionRead))
	resp := requestBuilds(t, ctx, apiRouter(t, nil), token, "GET", "/api/builds", "")
	require.Equal(t, http.StatusOK, resp.Code)
	body := decodeBuilds(t, resp)
	require.Len(t, body, 2)
	require.Equal(t, published.ID, body[0].ID)
	require.Equal(t, "succeeded", body[0].Status)
	require.Equal(t, "ghcr.io/prod9/later-app:v1.2.3", body[0].Modules[0].Image)
	require.Equal(t, "sha256:abc", body[0].Modules[0].Hash)
	require.Equal(t, queued.ID, body[1].ID)
	require.Equal(t, "queued", body[1].Status)
	require.Len(t, body[1].Modules, 2)
}

func TestListFiltersBeforeApplyingLimit(t *testing.T) {
	ctx := setupDB(t)
	visible := queueTestBuild(t, ctx, "app")
	for i := 0; i < listLimit; i++ {
		queueTestBuild(t, ctx, fmt.Sprintf("hidden-%d", i))
	}
	token := startTestSession(t, ctx, testRepository("app", auth.PermissionRead))
	router := apiRouter(t, nil)

	req := httptest.NewRequest("GET", "/api/builds", nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "platform_session", Value: token})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	body := decodeBuilds(t, resp)
	require.Len(t, body, 1)
	require.Equal(t, visible.ID, body[0].ID)
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

// Every selected module remains visible, even before its worker emits events.
func TestGetIncludesUnstartedModulesAndKeepsOutputBehindSteps(t *testing.T) {
	ctx := setupDB(t)
	build := queueTestBuild(t, ctx, "app")
	modules := modulesFor(t, ctx, build.ID)
	require.Len(t, modules, 2)
	claimTestModule(t, ctx, modules[0])
	appendTestEvents(t, ctx, modules[0].ID,
		&AppendEvent{Kind: EventConfigDone, At: at(1), EngineHost: "local"},
		&AppendEvent{Kind: EventStepStart, Step: "test", At: at(2)},
		&AppendEvent{Kind: EventStepDone, Step: "test", At: at(3), Error: "boom", Stdout: "private stdout", Stderr: "private stderr"},
		&AppendEvent{Kind: EventRunDone, At: at(4), Error: "boom"})
	token := startTestSession(t, ctx, testRepository("app", auth.PermissionRead))
	router := apiRouter(t, nil)
	for _, path := range []string{fmt.Sprintf("/api/builds/%d", build.ID), "/api/builds", "/api/repos/prod9/app/builds"} {
		resp := requestBuilds(t, ctx, router, token, "GET", path, "")
		require.Equal(t, http.StatusOK, resp.Code)
		var body listedBuild
		if path == fmt.Sprintf("/api/builds/%d", build.ID) {
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
		} else {
			listing := decodeBuilds(t, resp)
			require.Len(t, listing, 1)
			body = listing[0]
		}
		require.Equal(t, "running", body.Status)
		require.Equal(t, "boom", body.Error)
		require.True(t, body.FinishedAt.IsZero())
		require.Len(t, body.Modules, 2)
		require.Equal(t, modules[0].ID, body.Modules[0].BuildModuleID)
		require.Equal(t, "failed", body.Modules[0].Status)
		require.Equal(t, "worker-test", body.Modules[0].ClaimedBy)
		require.Equal(t, "local", body.Modules[0].EngineHost)
		require.Equal(t, at(1), body.Modules[0].EngineAssignedAt.UTC())
		require.Equal(t, "run_done", body.Modules[0].LastEventKind)
		require.Equal(t, at(4), body.Modules[0].LastEventAt.UTC())
		require.Equal(t, modules[1].ID, body.Modules[1].BuildModuleID)
		require.Equal(t, "queued", body.Modules[1].Status)
		require.NotContains(t, resp.Body.String(), "stdout")
		require.NotContains(t, resp.Body.String(), "stderr")
		require.NotContains(t, resp.Body.String(), "attempts")
	}
}

func TestGetInaccessibleBuildIsNotFound(t *testing.T) {
	ctx := setupDB(t)
	build := queueTestBuild(t, ctx, "app")
	token := startTestSession(t, ctx)
	router := apiRouter(t, nil)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/builds/%d", build.ID), nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "platform_session", Value: token})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNotFound, resp.Code)
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
	modules := modulesFor(t, ctx, build.ID)
	appendTestEvents(t, ctx, modules[1].ID, &AppendEvent{Kind: EventStepStart, Step: "test", At: at(1)})
	appendTestEvents(t, ctx, modules[0].ID, &AppendEvent{Kind: EventStepStart, Step: "test", At: at(2)},
		&AppendEvent{Kind: EventStepDone, Step: "test", At: at(3), Stdout: "ok\n", Stderr: "warn\n"})
	token := startTestSession(t, ctx, testRepository("app", auth.PermissionRead))
	resp := requestBuilds(t, ctx, apiRouter(t, nil), token, "GET", fmt.Sprintf("/api/builds/%d/steps", build.ID), "")
	require.Equal(t, http.StatusOK, resp.Code)
	var steps []struct {
		BuildModuleID int64     `json:"build_module_id"`
		Step          string    `json:"step"`
		Stdout        string    `json:"stdout"`
		Stderr        string    `json:"stderr"`
		FinishedAt    time.Time `json:"finished_at"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &steps))
	require.Len(t, steps, 2)
	require.Equal(t, modules[1].ID, steps[0].BuildModuleID)
	require.True(t, steps[0].FinishedAt.IsZero())
	require.Equal(t, modules[0].ID, steps[1].BuildModuleID)
	require.Equal(t, "test", steps[1].Step)
	require.Equal(t, "ok\n", steps[1].Stdout)
	require.Equal(t, "warn\n", steps[1].Stderr)
	require.NotContains(t, resp.Body.String(), "attempt")
	require.NotContains(t, resp.Body.String(), "unit")
}

func TestStepsForInaccessibleBuildAreNotFound(t *testing.T) {
	ctx := setupDB(t)
	build := queueTestBuild(t, ctx, "app")
	appendTestEvents(t, ctx, modulesFor(t, ctx, build.ID)[0].ID,
		&AppendEvent{Kind: EventStepStart, Step: "secret", At: at(1)})
	token := startTestSession(t, ctx)
	router := apiRouter(t, nil)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/builds/%d/steps", build.ID), nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "platform_session", Value: token})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNotFound, resp.Code)
	require.NotContains(t, resp.Body.String(), "secret")
}

// listedBuild mirrors the handler's wire shape; the fragment writes its own wire structs
// by hand (spec §No api/ contract layer), so the test asserts against a hand-written one.
type listedBuild struct {
	ID         int64          `json:"id"`
	Trigger    string         `json:"trigger"`
	RetryOf    int64          `json:"retry_of"`
	UserID     int64          `json:"user_id"`
	Owner      string         `json:"owner"`
	Repo       string         `json:"repo"`
	CloneURL   string         `json:"clone_url"`
	Ref        string         `json:"ref"`
	SHA        string         `json:"sha"`
	Status     string         `json:"status"`
	RepoID     int64          `json:"repo_id"`
	ManifestID int64          `json:"manifest_id"`
	FinishedAt time.Time      `json:"finished_at"`
	Modules    []listedModule `json:"modules"`
	Error      string         `json:"error"`
}

func decodeBuilds(t *testing.T, resp *httptest.ResponseRecorder) []listedBuild {
	body := []listedBuild{}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	return body
}

func appendTestEvents(t *testing.T, ctx context.Context, moduleID int64, records ...*AppendEvent) {
	for _, record := range records {
		record.BuildModuleID = moduleID
		require.NoError(t, record.Execute(ctx, nil))
	}
}

// eventsFor reads every recorded observation, including captured output.
func eventsFor(t *testing.T, ctx context.Context, buildID int64) []*BuildEvent {
	events := []*BuildEvent{}
	require.NoError(t, data.Select(ctx, &events, `SELECT e.* FROM build_events e
  JOIN build_modules bm ON bm.id = e.build_module_id WHERE bm.build_id = $1 ORDER BY e.id`, buildID))
	return events
}

type listedModule struct {
	BuildModuleID    int64     `json:"build_module_id"`
	ManifestModuleID int64     `json:"manifest_module_id"`
	Name             string    `json:"name"`
	Status           string    `json:"status"`
	Error            string    `json:"error"`
	Image            string    `json:"image"`
	Hash             string    `json:"hash"`
	ClaimedBy        string    `json:"claimed_by"`
	EngineHost       string    `json:"engine_host"`
	EngineAssignedAt time.Time `json:"engine_assigned_at"`
	LastEventKind    string    `json:"last_event_kind"`
	LastEventAt      time.Time `json:"last_event_at"`
}

func requestBuilds(t *testing.T, ctx context.Context, router chi.Router, token, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "platform_session", Value: token})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func TestTriggerSelectsModulesBeforeRecordingIntent(t *testing.T) {
	for _, test := range []struct {
		name    string
		field   string
		status  int
		modules int
	}{
		{"omitted", "", http.StatusCreated, 2},
		{"subset", `,"modules":["web"]`, http.StatusCreated, 1},
		{"empty", `,"modules":[]`, http.StatusBadRequest, 0},
		{"null", `,"modules":null`, http.StatusBadRequest, 0},
		{"duplicate", `,"modules":["api","api"]`, http.StatusBadRequest, 0},
		{"unknown", `,"modules":["missing"]`, http.StatusBadRequest, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cfg := setupInstalled(t)
			registerTestRepo(t, ctx, "prodigy9", "app")
			token := startTestSession(t, ctx, auth.Repository{GitHubID: 1, Owner: "prodigy9", Name: "app", Permission: auth.PermissionWrite})
			body := `{"owner":"prodigy9","repo":"app","ref":"refs/tags/v1.2.3"` + test.field + `}`
			resp := requestBuilds(t, ctx, apiRouter(t, cfg), token, "POST", "/api/builds", body)
			require.Equal(t, test.status, resp.Code, resp.Body.String())
			var count int
			require.NoError(t, data.Get(ctx, &count, `SELECT count(*) FROM builds`))
			if test.status != http.StatusCreated {
				require.Zero(t, count)
				require.NoError(t, data.Get(ctx, &count, `SELECT count(*) FROM build_modules`))
				require.Zero(t, count)
				return
			}
			require.Equal(t, 1, count)
			var created listedBuild
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &created))
			require.NotZero(t, created.RepoID)
			require.NotZero(t, created.ManifestID)
			require.Equal(t, "queued", created.Status)
			require.Len(t, created.Modules, test.modules)
			require.Equal(t, "queued", created.Modules[0].Status)
			if test.modules == 1 {
				require.Equal(t, "web", created.Modules[0].Name)
			}
		})
	}
}

func TestTriggerRejectsMissingOrMalformedManifest(t *testing.T) {
	for _, test := range []struct {
		name           string
		manifest       string
		upstreamStatus int
		responseStatus int
	}{
		{"missing", "", http.StatusNotFound, http.StatusConflict},
		{"malformed", "[invalid", http.StatusOK, http.StatusBadRequest},
		{"no modules", "repository = 'github.com/prodigy9/app'", http.StatusOK, http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cfg := setupInstalledManifest(t, test.manifest, test.upstreamStatus)
			registerTestRepo(t, ctx, "prodigy9", "app")
			token := startTestSession(t, ctx, auth.Repository{GitHubID: 1, Owner: "prodigy9", Name: "app", Permission: auth.PermissionWrite})
			resp := requestBuilds(t, ctx, apiRouter(t, cfg), token, "POST", "/api/builds",
				`{"owner":"prodigy9","repo":"app","ref":"refs/tags/v1.2.3"}`)
			require.Equal(t, test.responseStatus, resp.Code, resp.Body.String())
			var count int
			require.NoError(t, data.Get(ctx, &count, `SELECT count(*) FROM builds`))
			require.Zero(t, count)
		})
	}
}

func TestTriggerRetryRecordsANewBuildWithSelectedModules(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	parent := &Build{}
	create := &Create{Owner: "prodigy9", Repo: "app", Ref: "refs/tags/v1.2.3", SHA: "oldsha", UserID: 1, Trigger: TriggerWebUI}
	prepareCreate(t, ctx, create)
	require.NoError(t, create.Execute(ctx, parent))
	token := startTestSession(t, ctx, auth.Repository{GitHubID: 1, Owner: "prodigy9", Name: "app", Permission: auth.PermissionWrite})
	body := fmt.Sprintf(`{"owner":"prodigy9","repo":"app","ref":"refs/tags/v1.2.3","retry_of":%d,"modules":["api"]}`, parent.ID)
	resp := requestBuilds(t, ctx, apiRouter(t, cfg), token, "POST", "/api/builds", body)
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())
	var retry listedBuild
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &retry))
	require.NotEqual(t, parent.ID, retry.ID)
	require.Equal(t, parent.ID, retry.RetryOf)
	require.Equal(t, "retry", retry.Trigger)
	require.Equal(t, "e4c7a1d9", retry.SHA)
	require.Len(t, retry.Modules, 1)
	require.Equal(t, "api", retry.Modules[0].Name)
	require.Equal(t, "queued", retry.Status)
	require.Len(t, modulesFor(t, ctx, parent.ID), 2)
}
