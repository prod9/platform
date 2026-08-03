package srv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/fxtest"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/auth"
	"platform.prodigy9.co/srv/builds"
	"platform.prodigy9.co/srv/install"
	"platform.prodigy9.co/srv/migrate"
	"platform.prodigy9.co/srv/srvtest"
)

func TestRouterServesUIIndex(t *testing.T) {
	for _, installed := range []bool{false, true} {
		router, err := Router(fxtest.Configure(), nil, installed)
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, httptest.NewRequest("GET", "/", nil))

		require.Equal(t, http.StatusOK, resp.Code)
		require.Contains(t, resp.Body.String(), "platform")
	}
}

func TestRouterServesAPIHealth(t *testing.T) {
	for _, installed := range []bool{false, true} {
		router, err := Router(fxtest.Configure(), nil, installed)
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, httptest.NewRequest("GET", "/health", nil))

		require.Equal(t, http.StatusOK, resp.Code)
		require.Equal(t, "application/json", resp.Header().Get("Content-Type"))

		var health struct {
			Time time.Time `json:"time"`
		}
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &health))
		require.False(t, health.Time.IsZero())
	}
}

// The not-installed composition mounts the installer surface and no product /api/*;
// the installed composition is the reverse. The installer state read works with a nil
// DB (it reports db-reachable as an error), so this needs no postgres.
func TestNotInstalledMountsInstallerNotProduct(t *testing.T) {
	router, err := Router(fxtest.Configure(), nil, false)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, get(router, "/api/install").Code)
	require.Equal(t, http.StatusNotFound, get(router, "/api/builds").Code)
}

func TestInstalledMountsProductNotInstaller(t *testing.T) {
	router, err := Router(fxtest.Configure(), nil, true)
	require.NoError(t, err)

	require.Equal(t, http.StatusNotFound, get(router, "/api/install").Code)
	require.NotEqual(t, http.StatusNotFound, get(router, "/api/builds").Code)
}

// There is no generic settings REST surface in either composition — settings writes go
// through purpose-built installer actions only (platform-server.md §Operations).
func TestNoSettingsSurface(t *testing.T) {
	for _, installed := range []bool{false, true} {
		router, err := Router(fxtest.Configure(), nil, installed)
		require.NoError(t, err)

		require.Equal(t, http.StatusNotFound, get(router, "/api/settings").Code)
	}
}

// The wizard's credential step is ungated in the installer composition — no session
// can exist before the credentials enable login (installation.md). 400 (validation)
// proves the handler was reached, not a gate; the installed composition drops the
// route outright.
func TestCredentialsMountedUngatedInInstaller(t *testing.T) {
	notInstalled, err := Router(fxtest.Configure(), nil, false)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest,
		post(notInstalled, "/api/install/credentials", "{}").Code)

	installed, err := Router(fxtest.Configure(), nil, true)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound,
		post(installed, "/api/install/credentials", "{}").Code)
}

// One host serves module resolution and the product: any path with ?go-get=1 answers
// the go-import meta for platform.prodigy9.co in both compositions (platform-server.md
// §Operations; the standalone vanity command is gone).
func TestGoGetMetaServed(t *testing.T) {
	for _, installed := range []bool{false, true} {
		router, err := Router(fxtest.Configure(), nil, installed)
		require.NoError(t, err)

		for _, path := range []string{"/?go-get=1", "/framework?go-get=1"} {
			resp := get(router, path)
			require.Equal(t, http.StatusOK, resp.Code, path)
			require.Contains(t, resp.Body.String(), "go-import", path)
			require.Contains(t, resp.Body.String(), "platform.prodigy9.co", path)
			require.Contains(t, resp.Body.String(), "github.com/prod9/platform", path)
		}
	}
}

// The auth fragment mounts in both compositions — the org-owner claim needs a login
// before the server is installed (installation.md). 401 proves the route is mounted
// and gated; 404 would mean the composition dropped it.
func TestAuthMountsInBothCompositions(t *testing.T) {
	for _, installed := range []bool{false, true} {
		router, err := Router(fxtest.Configure(), nil, installed)
		require.NoError(t, err)

		require.Equal(t, http.StatusUnauthorized, get(router, "/api/session").Code)
	}
}

// A path with no prerendered file gets the SPA fallback at 404 — a wrong URL that
// answers 200 is a lie (spec §The status of a page is the server's answer).
func TestUIUnknownPathIsFallbackAt404(t *testing.T) {
	for _, installed := range []bool{false, true} {
		router, err := Router(fxtest.Configure(), nil, installed)
		require.NoError(t, err)

		resp := get(router, "/no/such/page")
		require.Equal(t, http.StatusNotFound, resp.Code)
		require.Contains(t, resp.Header().Get("Content-Type"), "text/html")
		require.Contains(t, resp.Body.String(), "<html")
	}
}

// The dynamic /builds/{id} route answers with the status the record deserves: 404 when
// the build does not exist, 200 when it does — the fallback supplies only the body.
func TestUIBuildRouteStatusFollowsTheRecord(t *testing.T) {
	ctx := srvtest.SetupDB(t,
		migrate.JobsTable,
		migrator.FromFS(auth.Migrations),
		migrator.FromFS(builds.Migrations),
		install.Source)
	systemUser, err := auth.SystemUserID(ctx)
	require.NoError(t, err)

	build := &builds.Build{}
	create := &builds.Create{Trigger: builds.TriggerWebUI, UserID: systemUser,
		Owner: "prod9", Repo: "app", CloneURL: "https://github.com/prod9/app.git",
		Ref: "refs/tags/v1.2.3", SHA: "abc123"}
	require.NoError(t, create.Execute(ctx, build))

	router, err := Router(fxtest.Configure(), nil, true)
	require.NoError(t, err)

	found := httptest.NewRecorder()
	router.ServeHTTP(found, httptest.NewRequest("GET",
		fmt.Sprintf("/builds/%d", build.ID), nil).WithContext(ctx))
	require.Equal(t, http.StatusOK, found.Code)
	require.Contains(t, found.Body.String(), "<html")

	// The SPA links with a trailing slash (trailingSlash = "always"), so the shape
	// matches with or without one.
	slashed := httptest.NewRecorder()
	router.ServeHTTP(slashed, httptest.NewRequest("GET",
		fmt.Sprintf("/builds/%d/", build.ID), nil).WithContext(ctx))
	require.Equal(t, http.StatusOK, slashed.Code)

	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, httptest.NewRequest("GET", "/builds/999999", nil).WithContext(ctx))
	require.Equal(t, http.StatusNotFound, missing.Code)
	require.Contains(t, missing.Body.String(), "<html")
}

func get(router http.Handler, path string) *httptest.ResponseRecorder {
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", path, nil))
	return resp
}

func post(router http.Handler, path, body string) *httptest.ResponseRecorder {
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("POST", path, strings.NewReader(body)))
	return resp
}
