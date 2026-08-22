package system

import (
	"context"
	"net/http"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/httpserver/render"
	"github.com/go-chi/chi/v5"
	"platform.prodigy9.co/srv/auth"
)

var (
	requireSystemUser = auth.RequireUser
	readSettings      = Settings
	readMigrations    = Migrations
	runMigrations     = func(ctx context.Context) error {
		return (&RunMigrations{}).Execute(ctx, nil)
	}
)

type SystemCtr struct{}

var _ controllers.Interface = SystemCtr{}

func (c SystemCtr) Mount(_ *config.Source, router chi.Router) error {
	router.Route("/api/system", func(router chi.Router) {
		router.Get("/settings", c.getSettings)
		router.Get("/migrations", c.getMigrations)
		router.Post("/migrations", c.runMigrations)
	})
	return nil
}

func (SystemCtr) getSettings(resp http.ResponseWriter, req *http.Request) {
	if _, ok := requireSystemUser(resp, req); !ok {
		return
	}

	sections, err := readSettings(req.Context())
	if err != nil {
		render.Error(resp, req, http.StatusInternalServerError, err)
		return
	}
	render.JSON(resp, req, sections)
}

func (SystemCtr) getMigrations(resp http.ResponseWriter, req *http.Request) {
	if _, ok := requireSystemUser(resp, req); !ok {
		return
	}

	plans, err := readMigrations(req.Context())
	if err != nil {
		render.Error(resp, req, http.StatusInternalServerError, err)
		return
	}
	render.JSON(resp, req, plans)
}

func (SystemCtr) runMigrations(resp http.ResponseWriter, req *http.Request) {
	if _, ok := requireSystemUser(resp, req); !ok {
		return
	}

	if err := runMigrations(req.Context()); err != nil {
		render.Error(resp, req, http.StatusInternalServerError, err)
		return
	}

	plans, err := readMigrations(req.Context())
	if err != nil {
		render.Error(resp, req, http.StatusInternalServerError, err)
		return
	}
	render.JSON(resp, req, plans)
}
