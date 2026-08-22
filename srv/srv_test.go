package srv

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fx.prodigy9.co/app"
	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/fxtest"
	"fx.prodigy9.co/secret"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/auth"
	"platform.prodigy9.co/srv/builds"
	"platform.prodigy9.co/srv/srvtest"
)

func init() {
	app.RegisterMigrations(App)
}

func TestAppCollectsPermanentServerConcerns(t *testing.T) {
	children := App.Children()
	require.Equal(t, []string{"github", "auth", "builds", "repos", "install", "system", "webui"},
		appNames(children))
	require.NotNil(t, children[1].EmbeddedMigrations())
	require.NotNil(t, children[2].EmbeddedMigrations())
	require.NotNil(t, children[3].EmbeddedMigrations())
	require.Len(t, children[2].Middlewares(), 2)
	require.Len(t, children[3].Middlewares(), 2)
	require.Len(t, children[4].Middlewares(), 1)

	jobs := app.CollectJobs(App)
	require.Len(t, jobs, 2)
	require.IsType(t, &builds.ScanBuilds{}, jobs[0])
	require.IsType(t, &builds.RunBuild{}, jobs[1])
	require.False(t, app.CollectFragment(App).IsEmpty())
}

func TestUIServesIndex(t *testing.T) {
	resp := get(uiRouter(t), "/")

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), "platform")
}

func TestUIHidesInstallerRouteAfterClaim(t *testing.T) {
	ctx := srvtest.SetupDB(t)
	seedClaim(t, ctx)

	resp := serve(ctx, uiRouter(t), "/install/")
	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestUIHidesProductRouteBeforeClaim(t *testing.T) {
	ctx := srvtest.SetupDB(t)

	resp := serve(ctx, uiRouter(t), "/builds/")
	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestHealth(t *testing.T) {
	resp := httptest.NewRecorder()
	health(resp, httptest.NewRequest(http.MethodGet, "/health", nil))

	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "application/json", resp.Header().Get("Content-Type"))

	var body struct {
		Time time.Time `json:"time"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.False(t, body.Time.IsZero())
}

func TestValidateBootRequiresSecretAfterClaim(t *testing.T) {
	ctx := srvtest.SetupDB(t)
	seedClaim(t, ctx)
	cfg := config.FromContext(ctx)
	config.Set(cfg, secret.SecretConfig, "")

	require.EqualError(t, ValidateBoot(ctx, cfg), "srv: SECRET must be set to boot the claimed server")
}

func TestValidateBootAllowsUnclaimedServerWithoutSecret(t *testing.T) {
	ctx := srvtest.SetupDB(t)
	cfg := config.FromContext(ctx)
	config.Set(cfg, secret.SecretConfig, "")

	require.NoError(t, ValidateBoot(ctx, cfg))
}

func TestValidateBootAllowsClaimedServerWithSecret(t *testing.T) {
	ctx := srvtest.SetupDB(t)
	seedClaim(t, ctx)
	cfg := config.FromContext(ctx)
	config.Set(cfg, secret.SecretConfig, "test-secret")

	require.NoError(t, ValidateBoot(ctx, cfg))
}

func TestValidateBootAllowsInvalidDatabaseURL(t *testing.T) {
	cfg := fxtest.Configure()
	config.Set(cfg, data.DatabaseURLConfig, "://invalid")

	require.NoError(t, ValidateBoot(t.Context(), cfg))
}

func TestValidateBootAllowsUnreachableDatabase(t *testing.T) {
	cfg := fxtest.Configure()
	config.Set(cfg, data.DatabaseURLConfig,
		"postgres://postgres@127.0.0.1:1/platform?sslmode=disable&connect_timeout=1")

	require.NoError(t, ValidateBoot(t.Context(), cfg))
}

// Module resolution is a static fact baked into every SPA shell, including the 404
// fallback parsed by the Go toolchain (platform-server.md; vendor/go-vanity-imports.md).
func TestGoImportMetaOnEveryPage(t *testing.T) {
	const meta = `<meta name="go-import" content="platform.prodigy9.co git https://github.com/prod9/platform" />`
	router := uiRouter(t)

	for path, status := range map[string]int{"/": http.StatusOK, "/no/such/page": http.StatusNotFound} {
		resp := get(router, path)
		require.Equal(t, status, resp.Code, path)
		require.Contains(t, resp.Body.String(), meta, path)
	}
}

func TestUIUnknownPathIsFallbackAt404(t *testing.T) {
	resp := get(uiRouter(t), "/no/such/page")

	require.Equal(t, http.StatusNotFound, resp.Code)
	require.Contains(t, resp.Header().Get("Content-Type"), "text/html")
	require.Contains(t, resp.Body.String(), "<html")
}

// The dynamic /builds/{id} route answers with the status the record deserves: 404 when
// the build does not exist, 200 when it does — the fallback supplies only the body.
func TestUIBuildRouteStatusFollowsTheRecord(t *testing.T) {
	ctx := srvtest.SetupDB(t)
	seedClaim(t, ctx)
	systemUser, err := auth.SystemUserID(ctx)
	require.NoError(t, err)

	build := &builds.Build{}
	create := &builds.Create{Trigger: builds.TriggerWebUI, UserID: systemUser,
		Owner: "prod9", Repo: "app", CloneURL: "https://github.com/prod9/app.git",
		Ref: "refs/tags/v1.2.3", SHA: "abc123"}
	require.NoError(t, create.Execute(ctx, build))

	router := uiRouter(t)
	found := serve(ctx, router, fmt.Sprintf("/builds/%d", build.ID))
	require.Equal(t, http.StatusOK, found.Code)
	require.Contains(t, found.Body.String(), "<html")

	slashed := serve(ctx, router, fmt.Sprintf("/builds/%d/", build.ID))
	require.Equal(t, http.StatusOK, slashed.Code)

	missing := serve(ctx, router, "/builds/999999")
	require.Equal(t, http.StatusNotFound, missing.Code)
	require.Contains(t, missing.Body.String(), "<html")
}

func seedClaim(t *testing.T, ctx context.Context) {
	require.NoError(t, srvtest.SeedSettings(ctx, map[string]string{
		"install.org_id":               "42",
		"install.org_login":            "prod9",
		"install.installation_id":      "84",
		"install.installed_by_user_id": "21",
		"install.installed_by_login":   "owner",
		"install.installed_at":         "2026-08-22T00:00:00Z",
	}))
}

func appNames(fragments []app.Interface) []string {
	names := make([]string, len(fragments))
	for i, fragment := range fragments {
		names[i] = fragment.Name()
	}
	return names
}

func uiRouter(t *testing.T) chi.Router {
	router := chi.NewRouter()
	require.NoError(t, (UI{}).Mount(fxtest.Configure(), router))
	return router
}

func get(handler http.Handler, path string) *httptest.ResponseRecorder {
	return serve(context.Background(), handler, path)
}

func serve(ctx context.Context, handler http.Handler, path string) *httptest.ResponseRecorder {
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
	handler.ServeHTTP(resp, req)
	return resp
}
