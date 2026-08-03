package srv

import (
	"fx.prodigy9.co/app/settings"
	"fx.prodigy9.co/config"
	"fx.prodigy9.co/httpserver/controllers"
	"github.com/go-chi/chi/v5"
	"platform.prodigy9.co/srv/auth"
)

// SettingsCtr mounts fx's settings app at /api/settings behind the session gate. fx
// ships settings.Ctr ungated by design — the embedder applies authorization — so the
// wrapper opens an inner router group carrying auth.Require and mounts the embedded
// controller there (docs/spec/platform-server.md §Operations).
type SettingsCtr struct {
	settings.Ctr
}

var _ controllers.Interface = SettingsCtr{}

func (c SettingsCtr) Mount(cfg *config.Source, router chi.Router) error {
	var err error
	router.Route("/api", func(api chi.Router) {
		// An inline group so auth.Require wraps only the settings routes — a subrouter-
		// wide Use would also answer unknown /api/* paths with 401, and the SPA reads
		// GET /api/install's plain 404 as the "installed" signal (installation.md).
		api.Group(func(g chi.Router) {
			g.Use(auth.Require)
			err = c.Ctr.Mount(cfg, g)
		})
	})
	return err
}
