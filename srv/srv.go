// Package srv is the platform server: the API + webhook processor layer above the
// shared build/render/publish packages (docs/spec/platform-server.md). It serves the
// embedded web UI at / and gates the API by install state — boot decides the
// composition once from install.GetState (docs/spec/installation.md): while the server
// is not completely installed it mounts only the installer fragment; once installed it
// mounts the product fragments (auth, builds) and starts the build runner. The server
// always boots — a DB unreachable is an install-state error, not a boot failure — and
// migrations never auto-run at boot.
package srv

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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
	"platform.prodigy9.co/engine"
	"platform.prodigy9.co/srv/auth"
	"platform.prodigy9.co/srv/builds"
	"platform.prodigy9.co/srv/install"
	"platform.prodigy9.co/srv/migrate"
)

// Serve configures and runs the platform server until interrupted, listening on
// httpserver.ListenAddrConfig (LISTEN_ADDR). It always boots: the DB is connected
// best-effort and its state, plus config and migration state, decides the install
// composition once (installer vs product fragments).
func Serve() error {
	cfg := config.Configure()

	db := connectOrNil(cfg)
	if db != nil {
		defer db.Close() // boot pool doubles as the runner's; AddDataContext owns HTTP's own
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

	// The build runner only matters once installed — there is no product traffic to
	// queue builds before then, and an uninstalled server may have no reachable DB.
	if installed {
		eng := engine.New(cfg)
		defer eng.Close()

		dataCtx := data.NewContext(context.Background(), db)
		runnerCtx, stopRunner := context.WithCancel(engine.NewContext(dataCtx, eng))
		runnerDone := make(chan struct{})
		go func() {
			defer close(runnerDone)
			builds.RunQueued(runnerCtx, cfg)
		}()
		defer func() { stopRunner(); <-runnerDone }() // before the db/engine Close defers
		ctrlc.Do(func() { stopRunner(); server.Close() })
	} else {
		ctrlc.Do(func() { server.Close() })
	}

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

	var ctrs []controllers.Interface
	if installed {
		ctrs = append(ctrs, auth.SessionCtr{}, builds.APICtr{}, builds.WebhookCtr{})
	} else {
		ctrs = append(ctrs, install.StateCtr{DB: db, Merged: merged})
	}
	ctrs = append(ctrs, UI{}) // catch-all /* — mounts last

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
	migrator.FromFS(auth.Migrations),
	migrator.FromFS(builds.Migrations),
	migrator.FromFS(install.Migrations),
)
