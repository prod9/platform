// Package srv is the platform server: the API + webhook processor layer above the
// shared build/render/publish packages (docs/spec/platform-server.md). It serves the
// embedded web UI at / and gates the API by install state. Installer and product
// fragments stay mounted; the gate changes behavior after claim. Executing queued builds
// belongs to a worker peer, not to this process (docs/spec/platform-server.md). The
// server always boots — a DB unreachable is an install-state error, not a boot failure
// — and migrations never auto-run at boot.
package srv

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fx.prodigy9.co/app"
	fxcmd "fx.prodigy9.co/cmd"
	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/fxlog"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/httpserver/httperrors"
	"fx.prodigy9.co/httpserver/render"
	"fx.prodigy9.co/secret"
	"github.com/go-chi/chi/v5"
	"platform.prodigy9.co/srv/auth"
	"platform.prodigy9.co/srv/builds"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/install"
	"platform.prodigy9.co/srv/repos"
	"platform.prodigy9.co/srv/system"
	"platform.prodigy9.co/webui"
)

// App is the permanent fx application tree consumed by Platform's Cobra root through
// fx's public collectors. Install state changes which routes are reachable, never which
// fragments are composed.
var App app.Interface = app.Build().
	Name("srv").
	AddDefaultMiddlewares().
	Command(fxcmd.BuildDataCommand()).
	Mount(github.Fragment).
	Mount(auth.App).
	Mount(builds.App).
	Mount(repos.App).
	Mount(install.App).
	Mount(system.App).
	Mount(app.Build().
		Name("webui").
		Controllers(controllers.FromFunc("/health", health), UI{})).
	App()

// ValidateBoot enforces boot-only configuration that depends on durable install state.
// An unavailable database remains a request-time condition, so it is logged and softened.
func ValidateBoot(ctx context.Context, cfg *config.Source) (err error) {
	db, err := data.Connect(cfg)
	if err != nil {
		fxlog.Log("database unavailable at boot", fxlog.String("error", err.Error()))
		return nil
	}
	defer func() {
		err = errors.Join(err, db.Close())
	}()

	ctx = data.NewContext(config.NewContext(ctx, cfg), db)
	if err := db.PingContext(ctx); err != nil {
		fxlog.Log("database unavailable at boot", fxlog.String("error", err.Error()))
		return nil
	}

	installed, err := install.IsInstalled(ctx, db)
	if err != nil {
		return err
	}
	if installed {
		if _, ok := config.GetOK(cfg, secret.SecretConfig); !ok {
			return errors.New("srv: SECRET must be set to boot the claimed server")
		}
	}
	return nil
}

func health(resp http.ResponseWriter, req *http.Request) {
	render.JSON(resp, req, struct {
		Time time.Time `json:"time"`
	}{time.Now()})
}

// UI serves the embedded web UI (webui.Assets) at the site root; requests not matched
// by an API route fall through to it. The server decides every page's status itself
// (docs/spec/platform-server.md §The status of a page is the server's answer): a
// prerendered file is served as-is, the known dynamic route /builds/{id} gets the SPA
// fallback at the status the record deserves, and anything unrecognized gets the
// fallback at 404.
type UI struct{}

var _ controllers.Interface = UI{}

func (UI) Mount(_ *config.Source, router chi.Router) error {
	build, err := fs.Sub(webui.Assets, "build")
	if err != nil {
		return err
	}
	fallback, err := fs.ReadFile(build, "fallback.html")
	if err != nil {
		return err
	}

	files := http.FileServer(http.FS(build))
	router.Handle("/*", http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		prerenderedPath := prerendered(build, req.URL.Path)
		installerPath := installRoute(req.URL.Path)
		_, dynamicBuildPath := buildRoute(req.URL.Path)
		productPath := dynamicBuildPath || productPage(req.URL.Path, prerenderedPath)
		if installerPath || productPath {
			db, ok := data.LookupFromContext(req.Context())
			if !ok {
				render.Error(resp, req, http.StatusInternalServerError, errors.New("srv: UI route requires database context"))
				return
			}
			installed, err := install.IsInstalled(req.Context(), db)
			if err != nil {
				render.Error(resp, req, http.StatusInternalServerError, err)
				return
			}
			hideInstaller := installerPath && installed
			hideProduct := productPath && !installed
			if hideInstaller || hideProduct {
				render.Error(resp, req, http.StatusNotFound, httperrors.ErrNotFound)
				return
			}
		}

		if prerenderedPath {
			files.ServeHTTP(resp, req)
			return
		}

		status := http.StatusNotFound
		if id, ok := buildRoute(req.URL.Path); ok {
			exists, err := builds.Exists(req.Context(), id)
			if err != nil {
				render.Error(resp, req, 500, err)
				return
			}
			if exists {
				status = http.StatusOK
			}
		}

		resp.Header().Set("Content-Type", "text/html; charset=utf-8")
		resp.WriteHeader(status)
		if _, err := resp.Write(fallback); err != nil {
			fxlog.Error(err)
		}
	}))
	return nil
}

func installRoute(path string) bool {
	return path == "/install" || strings.HasPrefix(path, "/install/")
}

func productPage(path string, prerendered bool) bool {
	if !prerendered || path == "/" || strings.HasPrefix(path, "/_app/") {
		return false
	}
	return !installRoute(path)
}

// prerendered reports whether the embedded build output holds a file for the path — a
// file itself, or a directory page (adapter-static emits route/index.html, which the
// file server resolves).
func prerendered(build fs.FS, path string) bool {
	name := strings.Trim(path, "/")
	if name == "" {
		name = "index.html"
	}

	info, err := fs.Stat(build, name)
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return true
	}
	_, err = fs.Stat(build, name+"/index.html")
	return err == nil
}

// buildRoute matches the webui's one dynamic route shape, /builds/{id} — with or
// without a trailing slash, since the SPA links with one (trailingSlash = "always").
// Knowing the shape is the price of a static UI answering with a real status.
func buildRoute(path string) (int64, bool) {
	rest, ok := strings.CutPrefix(path, "/builds/")
	rest = strings.TrimSuffix(rest, "/")
	if !ok || strings.Contains(rest, "/") {
		return 0, false
	}

	id, err := strconv.ParseInt(rest, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}
