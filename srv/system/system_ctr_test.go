package system

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"fx.prodigy9.co/fxtest"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/auth"
)

func TestSystemRoutesRequireSession(t *testing.T) {
	router := chi.NewRouter()
	require.NoError(t, SystemCtr{}.Mount(fxtest.Configure(), router))

	for _, request := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/system/settings"},
		{http.MethodGet, "/api/system/migrations"},
		{http.MethodPost, "/api/system/migrations"},
	} {
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, httptest.NewRequest(request.method, request.path, nil))
		require.Equal(t, http.StatusUnauthorized, resp.Code, "%s %s", request.method, request.path)
	}
}

func TestSystemGetRoutesRenderDomainReads(t *testing.T) {
	stubSystemUser(t)
	readSettings = func(context.Context) ([]SettingSection, error) {
		return []SettingSection{{Name: "Server", Facts: []SettingFact{{Key: "org", Value: "prod9"}}}}, nil
	}
	readMigrations = func(context.Context) ([]MigrationPlan, error) {
		return []MigrationPlan{{Action: "migrate", Migration: "202608221000_create_repos"}}, nil
	}
	t.Cleanup(resetControllerSeams)
	router := mountSystemController(t)

	settingsResp := requestSystem(router, http.MethodGet, "/api/system/settings")
	require.Equal(t, http.StatusOK, settingsResp.Code)
	var sections []SettingSection
	require.NoError(t, json.Unmarshal(settingsResp.Body.Bytes(), &sections))
	require.Equal(t, "prod9", sections[0].Facts[0].Value)

	migrationsResp := requestSystem(router, http.MethodGet, "/api/system/migrations")
	require.Equal(t, http.StatusOK, migrationsResp.Code)
	var plans []MigrationPlan
	require.NoError(t, json.Unmarshal(migrationsResp.Body.Bytes(), &plans))
	require.Equal(t, "202608221000_create_repos", plans[0].Migration)
}

func TestSystemPostRunsThenReturnsFreshPlan(t *testing.T) {
	stubSystemUser(t)
	run := false
	runMigrations = func(context.Context) error {
		run = true
		return nil
	}
	readMigrations = func(context.Context) ([]MigrationPlan, error) {
		require.True(t, run)
		return []MigrationPlan{}, nil
	}
	t.Cleanup(resetControllerSeams)

	resp := requestSystem(mountSystemController(t), http.MethodPost, "/api/system/migrations")

	require.Equal(t, http.StatusOK, resp.Code)
	require.JSONEq(t, `[]`, resp.Body.String())
}

func TestSystemRoutesReportDomainFailures(t *testing.T) {
	stubSystemUser(t)
	domainErr := errors.New("domain failed")
	readSettings = func(context.Context) ([]SettingSection, error) { return nil, domainErr }
	readMigrations = func(context.Context) ([]MigrationPlan, error) { return nil, domainErr }
	runMigrations = func(context.Context) error { return domainErr }
	t.Cleanup(resetControllerSeams)
	router := mountSystemController(t)

	for _, request := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/system/settings"},
		{http.MethodGet, "/api/system/migrations"},
		{http.MethodPost, "/api/system/migrations"},
	} {
		resp := requestSystem(router, request.method, request.path)
		require.Equal(t, http.StatusInternalServerError, resp.Code, "%s %s", request.method, request.path)
	}
}

func stubSystemUser(t *testing.T) {
	t.Helper()
	requireSystemUser = func(http.ResponseWriter, *http.Request) (*auth.User, bool) {
		return &auth.User{}, true
	}
}

func mountSystemController(t *testing.T) chi.Router {
	t.Helper()
	router := chi.NewRouter()
	require.NoError(t, SystemCtr{}.Mount(fxtest.Configure(), router))
	return router
}

func requestSystem(router chi.Router, method, path string) *httptest.ResponseRecorder {
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(method, path, nil))
	return resp
}

func resetControllerSeams() {
	requireSystemUser = auth.RequireUser
	readSettings = Settings
	readMigrations = Migrations
	runMigrations = func(ctx context.Context) error {
		return (&RunMigrations{}).Execute(ctx, nil)
	}
}
