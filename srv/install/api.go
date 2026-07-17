package install

import (
	"net/http"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/httpserver/render"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"platform.prodigy9.co/srv/migrate"
)

// StateCtr serves the gated installer surface: the ordered install-state read and the
// migrations remediation. Boot mounts it only while the server is not completely
// installed, so its absence (a 404 on GET /api/install) is the SPA's "installed" signal.
type StateCtr struct {
	DB     *sqlx.DB
	Merged migrator.Source
}

var _ controllers.Interface = StateCtr{}

func (c StateCtr) Mount(cfg *config.Source, router chi.Router) error {
	router.Get("/api/install", c.getState)
	router.Post("/api/install/migrations", c.runMigrations)
	return nil
}

func (c StateCtr) getState(resp http.ResponseWriter, req *http.Request) {
	render.JSON(resp, req, GetState(req.Context(), c.DB, c.Merged))
}

func (c StateCtr) runMigrations(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if err := migrate.Run(data.NewContext(ctx, c.DB), c.DB, c.Merged); err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	render.JSON(resp, req, GetState(ctx, c.DB, c.Merged))
}
