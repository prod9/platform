// Package srv is the platform server: the API + webhook processor layer above the
// shared build/render/publish packages (docs/spec/platform-server.md). It serves the
// embedded web UI at / and gates the API by install state — boot decides the
// composition once from install.GetState (docs/spec/installation.md): while the server
// is not completely installed it mounts only the installer fragment; once installed it
// mounts the product fragments (auth, builds). Executing queued builds belongs to a
// worker peer, not to this process (docs/spec/platform-server.md). The server always
// boots — a DB unreachable is an install-state error, not a boot failure — and
// migrations never auto-run at boot.
package srv

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/ctrlc"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/fxlog"
	"fx.prodigy9.co/httpserver"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/httpserver/middlewares"
	"fx.prodigy9.co/httpserver/render"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"platform.prodigy9.co/srv/auth"
	"platform.prodigy9.co/srv/builds"
	"platform.prodigy9.co/srv/install"
	"platform.prodigy9.co/srv/migrate"
	"platform.prodigy9.co/webui"
)

// Serve configures and runs the platform server until interrupted, listening on
// httpserver.ListenAddrConfig (LISTEN_ADDR). It always boots: the DB is connected
// best-effort and its state, plus config and migration state, decides the install
// composition once (installer vs product fragments).
func Serve() error {
	cfg := config.Configure()

	db := connectOrNil(cfg)
	if db != nil {
		defer db.Close() // boot pool only — AddDataContext owns HTTP's own
	}

	entries := install.GetState(config.NewContext(context.Background(), cfg), db, merged)
	installed := install.Complete(entries)

	router, err := Router(cfg, db, installed)
	if err != nil {
		return err
	}

	handler := http.Handler(router)
	if db != nil {
		handler = middlewares.AddDataContext(cfg)(router)
	}

	listenAddr := config.Get(cfg, httpserver.ListenAddrConfig)
	server := &http.Server{Addr: listenAddr, Handler: handler}
	ctrlc.Do(func() { server.Close() })

	fxlog.Log("listening", fxlog.String("addr", listenAddr), fxlog.Bool("installed", installed))
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Router builds the server's routes on a fresh chi router for the given install
// decision; Serve listens with it and tests drive it directly. Both compositions serve
// /health and the webui at /*; installed mounts the product fragments, not-installed
// mounts only the installer. db is passed to the installer controller (it may be nil).
func Router(cfg *config.Source, db *sqlx.DB, installed bool) (chi.Router, error) {
	router := chi.NewRouter()
	router.Use(middlewares.Configure(cfg))
	router.Use(middlewares.LogRequests(cfg))
	router.Get("/health", health)

	// Auth mounts in both compositions: the org-owner claim needs a login before the
	// server is installed (docs/spec/installation.md).
	ctrs := []controllers.Interface{auth.SessionCtr{}}
	if installed {
		ctrs = append(ctrs, SettingsCtr{})
	} else {
		ctrs = append(ctrs, install.StateCtr{DB: db, Merged: merged})
	}

	// Product fragments mount behind the install-record middleware — the bound record
	// is ambient truth for every product route (docs/spec/installation.md).
	if installed {
		product := router.With(install.RecordContext)
		for _, ctr := range []controllers.Interface{builds.BuildCtr{}, builds.WebhookCtr{}} {
			if err := ctr.Mount(cfg, product); err != nil {
				return nil, err
			}
		}
	}

	// Catch-all /* mounts last; the record lookup for dynamic-route statuses is wired
	// only when the product fragments are — the installer composition 404s them all.
	ui := UI{}
	if installed {
		ui.BuildExists = builds.Exists
	}
	ctrs = append(ctrs, ui)
	for _, ctr := range ctrs {
		if err := ctr.Mount(cfg, router); err != nil {
			return nil, err
		}
	}
	return router, nil
}

func health(resp http.ResponseWriter, req *http.Request) {
	render.JSON(resp, req, struct {
		Time time.Time `json:"time"`
	}{time.Now()})
}

// connectOrNil connects the boot DB pool best-effort: an unset DATABASE_URL or an
// unreachable database is a soft nil (the server still boots and serves the installer,
// which surfaces the condition as a db-reachable error), not a fatal boot error.
func connectOrNil(cfg *config.Source) *sqlx.DB {
	db, err := connectDB(cfg)
	if err != nil {
		fxlog.Log("database unavailable at boot; serving installer only",
			fxlog.String("error", err.Error()))
		return nil
	}
	return db
}

// connectDB opens and verifies the boot DB pool. A missing DATABASE_URL or an
// unreachable database is reported as an error for connectOrNil to soften.
func connectDB(cfg *config.Source) (*sqlx.DB, error) {
	if _, ok := config.GetOK(cfg, data.DatabaseURLConfig); !ok {
		return nil, errors.New("srv: DATABASE_URL is not set")
	}

	db, err := data.Connect(cfg)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("srv: database unreachable: %w", err)
	}
	return db, nil
}

// merged aggregates every fragment's embedded SQL — the srv-side equivalent of fx's
// Mount collecting fragment migrations. Migrations run via the installer or the CLI,
// never at boot (docs/spec/installation.md).
var merged = migrate.Merged(
	migrate.JobsTable,
	migrator.FromFS(auth.Migrations),
	migrator.FromFS(builds.Migrations),
	install.Source,
)

// UI serves the embedded web UI (webui.Assets) at the site root; requests not matched
// by an API route fall through to it. The server decides every page's status itself
// (docs/spec/platform-server.md §The status of a page is the server's answer): a
// prerendered file is served as-is, the known dynamic route /builds/{id} gets the SPA
// fallback at the status the record deserves, and anything unrecognized gets the
// fallback at 404. BuildExists is that record lookup — nil (the installer composition)
// makes every dynamic path a 404.
type UI struct {
	BuildExists func(ctx context.Context, id int64) (bool, error)
}

var _ controllers.Interface = UI{}

func (ui UI) Mount(cfg *config.Source, router chi.Router) error {
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
		if prerendered(build, req.URL.Path) {
			files.ServeHTTP(resp, req)
			return
		}

		status := http.StatusNotFound
		if id, ok := buildRoute(req.URL.Path); ok && ui.BuildExists != nil {
			exists, err := ui.BuildExists(req.Context(), id)
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
		resp.Write(fallback)
	}))
	return nil
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
