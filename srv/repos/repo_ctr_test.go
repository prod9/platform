package repos

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
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/srv/auth"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/install"
	"platform.prodigy9.co/srv/srvtest"
)

func init() {
	app.RegisterMigrations(App.App())
	app.RegisterMigrations(auth.App.App())
	app.RegisterMigrations(install.App.App())
}

func apiRouter(t *testing.T, cfg *config.Source) chi.Router {
	if cfg == nil {
		cfg = fxtest.Configure()
	}

	router := chi.NewRouter()
	router.Use(middlewares.Configure(cfg))
	require.NoError(t, RepoCtr{}.Mount(cfg, router))
	return router
}

// setupInstalled is an installed server for the registration endpoints: migrated DB
// with the install.* settings claimed, stubbed App credentials, and a fake GitHub
// serving token mints, the installation repo list, the user-scoped repo list, the
// repo lookup, and the manifest read.
func setupInstalled(t *testing.T) (context.Context, *config.Source) {
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
	mux.HandleFunc("GET /installation/repositories", func(resp http.ResponseWriter, req *http.Request) {
		require.Equal(t, "Bearer ghs_tok", req.Header.Get("Authorization"))
		fmt.Fprint(resp, `{"total_count":2,"repositories":[
			{"name":"app","full_name":"prodigy9/app","owner":{"login":"prodigy9"}},
			{"name":"api","full_name":"prodigy9/api","owner":{"login":"prodigy9"}}]}`)
	})
	// The user reaches only "app" — "api" is registered but must stay invisible.
	mux.HandleFunc("GET /user/installations/7/repositories", func(resp http.ResponseWriter, req *http.Request) {
		require.Equal(t, "Bearer gho_usertoken", req.Header.Get("Authorization"))
		fmt.Fprint(resp, `{"total_count":1,"repositories":[
			{"name":"app","full_name":"prodigy9/app","owner":{"login":"prodigy9"}}]}`)
	})
	mux.HandleFunc("GET /repos/prodigy9/app/contents/platform.toml", func(resp http.ResponseWriter, req *http.Request) {
		require.Equal(t, "Bearer ghs_tok", req.Header.Get("Authorization"))
		require.Equal(t, "abc123", req.URL.Query().Get("ref"))
		fmt.Fprint(resp, "repository = \"github.com/prodigy9/app\"\n\n[server]\npublish = \"never\"\n\n[modules.web]\nframework = \"pnpm/basic\"\n")
	})
	mux.HandleFunc("GET /repos/prodigy9/app/commits/main", func(resp http.ResponseWriter, req *http.Request) {
		require.Equal(t, "application/vnd.github.sha", req.Header.Get("Accept"))
		fmt.Fprint(resp, "abc123")
	})
	mux.HandleFunc("GET /repos/prodigy9/app", func(resp http.ResponseWriter, req *http.Request) {
		fmt.Fprint(resp, `{"clone_url":"https://github.com/prodigy9/app.git","default_branch":"main"}`)
	})
	mux.HandleFunc("GET /repos/prodigy9/api/commits/main", func(resp http.ResponseWriter, req *http.Request) {
		fmt.Fprint(resp, "def456")
	})
	mux.HandleFunc("GET /repos/prodigy9/api", func(resp http.ResponseWriter, req *http.Request) {
		fmt.Fprint(resp, `{"clone_url":"https://github.com/prodigy9/api.git","default_branch":"main"}`)
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

// startTestSession seeds a GitHub-linked user with a live session — the identity
// carries the encrypted OAuth token the visibility read reveals.
func startTestSession(t *testing.T, ctx context.Context) (int64, string) {
	upsert, user := &auth.UpsertGitHubUser{
		Account: auth.GitHubAccount{ID: 12345, Login: "octocat", Email: "octo@example.com"},
		Token:   "gho_usertoken",
	}, &auth.User{}
	require.NoError(t, upsert.Execute(ctx, user))

	token := "test-session-token"
	create := &auth.CreateSession{UserID: user.ID, Token: token, ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, create.Execute(ctx, nil))
	return user.ID, token
}

func registerTestRepo(t *testing.T, ctx context.Context, owner, repo string, userID int64) {
	raw := fmt.Sprintf("repository = %q\n\n[modules.app]\nframework = \"go/basic\"\n", "github.com/"+owner+"/"+repo)
	model, err := conf.Parse([]byte(raw))
	require.NoError(t, err)

	action := &RegisterRepo{
		Owner: owner, Repo: repo, UserID: userID,
		ManifestSHA: "seed-" + repo, ManifestRaw: raw, Manifest: *model,
	}
	require.NoError(t, action.Execute(ctx, &Repo{}))
}

func doRequest(t *testing.T, router chi.Router, ctx context.Context, session, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
	if session != "" {
		req.AddCookie(&http.Cookie{Name: "platform_session", Value: session})
	}
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func TestReposWithoutCookie(t *testing.T) {
	router := apiRouter(t, nil)

	resp := doRequest(t, router, context.Background(), "", "GET", "/api/repos", "")

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

// GET /api/repos is registered∩live: "api" is registered but out of the user's reach,
// "app" is both; only "app" answers. An unregistered reachable repo never appears.
func TestReposIntersectsRegisteredWithLive(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	userID, session := startTestSession(t, ctx)
	router := apiRouter(t, cfg)

	for _, name := range []string{"app", "api"} {
		registerTestRepo(t, ctx, "prodigy9", name, userID)
	}

	resp := doRequest(t, router, ctx, session, "GET", "/api/repos", "")
	require.Equal(t, http.StatusOK, resp.Code)

	var listing []struct {
		Owner    string `json:"owner"`
		Repo     string `json:"repo"`
		FullName string `json:"full_name"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &listing))
	require.Len(t, listing, 1)
	require.Equal(t, "prodigy9", listing[0].Owner)
	require.Equal(t, "app", listing[0].Repo)
	require.Equal(t, "prodigy9/app", listing[0].FullName)
}

func TestCandidatesExcludesRegistered(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	userID, session := startTestSession(t, ctx)
	router := apiRouter(t, cfg)

	registerTestRepo(t, ctx, "prodigy9", "app", userID)

	resp := doRequest(t, router, ctx, session, "GET", "/api/repos/candidates", "")
	require.Equal(t, http.StatusOK, resp.Code)

	var listing []struct {
		Owner    string `json:"owner"`
		Repo     string `json:"repo"`
		FullName string `json:"full_name"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &listing))
	require.Len(t, listing, 1)
	require.Equal(t, "api", listing[0].Repo)
}

func TestRegisterRepoRecordsAndConflicts(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	userID, session := startTestSession(t, ctx)
	router := apiRouter(t, cfg)

	resp := doRequest(t, router, ctx, session, "POST", "/api/repos",
		`{"owner":"prodigy9","repo":"app","manifest_sha":"abc123"}`)
	require.Equal(t, http.StatusCreated, resp.Code)

	row := &Repo{}
	require.NoError(t, data.Get(ctx, row, `SELECT * FROM repos WHERE owner = 'prodigy9' AND repo = 'app'`))
	require.Equal(t, userID, row.RegisteredBy)

	var snapshot struct {
		SHA        string `db:"sha"`
		Raw        string `db:"raw"`
		Repository string `db:"repository"`
		Publish    string `db:"server_publish"`
	}
	require.NoError(t, data.Get(ctx, &snapshot, `
		SELECT sha, raw, repository, server_publish
		FROM repo_manifests WHERE repo_id = $1`, row.ID))
	require.Equal(t, "abc123", snapshot.SHA)
	require.Contains(t, snapshot.Raw, `[modules.web]`)
	require.Equal(t, "github.com/prodigy9/app", snapshot.Repository)
	require.Equal(t, "never", snapshot.Publish)

	var module struct {
		Name      string `db:"name"`
		Framework string `db:"framework"`
		WorkDir   string `db:"workdir"`
	}
	require.NoError(t, data.Get(ctx, &module, `
		SELECT name, framework, workdir FROM repo_manifest_modules`))
	require.Equal(t, "web", module.Name)
	require.Equal(t, "pnpm/basic", module.Framework)
	require.Equal(t, ".", module.WorkDir)

	resp = doRequest(t, router, ctx, session, "POST", "/api/repos",
		`{"owner":"prodigy9","repo":"app","manifest_sha":"abc123"}`)
	require.Equal(t, http.StatusConflict, resp.Code)
}

// A repo the installation cannot see must not register — the same 404 boundary the
// manual build trigger draws.
func TestRegisterRepoUnreachable(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	_, session := startTestSession(t, ctx)
	router := apiRouter(t, cfg)

	resp := doRequest(t, router, ctx, session, "POST", "/api/repos",
		`{"owner":"prodigy9","repo":"ghost","manifest_sha":"abc123"}`)
	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestRegisterRepoWithoutManifest(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	_, session := startTestSession(t, ctx)
	router := apiRouter(t, cfg)

	resp := doRequest(t, router, ctx, session, "POST", "/api/repos",
		`{"owner":"prodigy9","repo":"api","manifest_sha":"def456"}`)
	require.Equal(t, http.StatusConflict, resp.Code)
}

func TestManifestParsesPlatformTOML(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	_, session := startTestSession(t, ctx)
	router := apiRouter(t, cfg)

	resp := doRequest(t, router, ctx, session, "GET", "/api/repos/prodigy9/app/manifest", "")
	require.Equal(t, http.StatusOK, resp.Code)

	var manifest struct {
		SHA        string `json:"sha"`
		Maintainer string `json:"maintainer"`
		Repository string `json:"repository"`
		Publish    string `json:"server_publish"`
		Modules    []struct {
			Name      string `json:"name"`
			Framework string `json:"framework"`
			WorkDir   string `json:"workdir"`
		} `json:"modules"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &manifest))
	require.Equal(t, "abc123", manifest.SHA)
	require.Equal(t, "github.com/prodigy9/app", manifest.Repository)
	require.Equal(t, "never", manifest.Publish)
	require.Len(t, manifest.Modules, 1)
	require.Equal(t, "web", manifest.Modules[0].Name)
	require.Equal(t, "pnpm/basic", manifest.Modules[0].Framework)
	require.Equal(t, ".", manifest.Modules[0].WorkDir)
}

func TestManifestAbsentIsConflict(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	_, session := startTestSession(t, ctx)
	router := apiRouter(t, cfg)

	resp := doRequest(t, router, ctx, session, "GET", "/api/repos/prodigy9/api/manifest", "")
	require.Equal(t, http.StatusConflict, resp.Code)
}

func TestManifestUnreachableIsNotFound(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	_, session := startTestSession(t, ctx)
	router := apiRouter(t, cfg)

	resp := doRequest(t, router, ctx, session, "GET", "/api/repos/prodigy9/ghost/manifest", "")
	require.Equal(t, http.StatusNotFound, resp.Code)
}
