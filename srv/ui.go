package srv

import (
	"io/fs"
	"net/http"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/httpserver/controllers"
	"github.com/go-chi/chi/v5"
	"platform.prodigy9.co/webui"
)

// UI serves the embedded web UI (webui.Assets) at the site root; requests not matched
// by an API route fall through to it.
type UI struct{}

var _ controllers.Interface = UI{}

func (UI) Mount(cfg *config.Source, router chi.Router) error {
	build, err := fs.Sub(webui.Assets, "build")
	if err != nil {
		return err
	}

	router.Handle("/*", http.FileServer(http.FS(build)))
	return nil
}
