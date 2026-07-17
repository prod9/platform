package install

import (
	"errors"
	"net/http"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/httpserver/render"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"platform.prodigy9.co/srv/migrate"
)

var errNoDB = errors.New("install: no database configured")

// StateCtr serves the gated installer surface: the ordered install-state read and the
// migrations remediation. Boot mounts it only while the server is not completely
// installed, so its absence (a 404 on GET /api/install) is the SPA's "installed" signal.
//
// It carries the boot DB handle (possibly nil) and the merged migration set explicitly
// rather than reading them from AddDataContext: the installer runs before that middleware
// is wired (it is added only once a DB exists), so it cannot rely on request-scoped data.
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
	if c.DB == nil {
		render.Error(resp, req, 503, errNoDB)
		return
	}

	ctx := req.Context()
	if err := migrate.Run(ctx, c.DB, c.Merged); err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	render.JSON(resp, req, GetState(ctx, c.DB, c.Merged))
}
