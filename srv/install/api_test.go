package install

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/fxtest"
	"fx.prodigy9.co/httpserver/middlewares"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/migrate"
	"platform.prodigy9.co/srv/srvtest"
)

func TestGetInstallReturnsOrderedEntries(t *testing.T) {
	ctx := srvtest.SetupDB(t, migrator.FromFS(Migrations))
	db := data.FromContext(ctx)

	cfg := fxtest.Configure()
	router := chi.NewRouter()
	router.Use(middlewares.Configure(cfg))
	ctr := StateCtr{DB: db, Merged: migrate.Merged(migrator.FromFS(Migrations))}
	require.NoError(t, ctr.Mount(cfg, router))

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/api/install", nil))
	require.Equal(t, http.StatusOK, resp.Code)

	var entries []Entry
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &entries))
	require.Equal(t,
		[]string{"db-reachable", "app-credentials", "app-installed", "migrations"},
		names(entries))
}
