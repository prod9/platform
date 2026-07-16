// Package srv is the platform server: the API + webhook processor layer above the
// shared build/render/publish packages (docs/spec/platform-server.md). It serves the
// embedded web UI at / and the API under /api/.
package srv

import (
	"context"
	"errors"
	"net/http"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/ctrlc"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/fxlog"
	"fx.prodigy9.co/httpserver"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/httpserver/middlewares"
	"github.com/go-chi/chi/v5"
	"platform.prodigy9.co/engine"
)

// Serve configures and runs the platform server until interrupted, listening on
// httpserver.ListenAddrConfig (LISTEN_ADDR).
func Serve() error {
	cfg := config.Configure()
	router, err := Router(cfg)
	if err != nil {
		return err
	}

	db, err := connectDB(cfg)
	if err != nil {
		return err
	}
	defer db.Close() // boot pool doubles as the runner's; AddDataContext owns HTTP's own
	if err := migrate(context.Background(), db); err != nil {
		return err
	}

	// running rows are orphans at boot: single-server model, so a live claimant from a
	// previous process cannot exist.
	dataCtx := data.NewContext(context.Background(), db)
	if err := (&RequeueOrphanBuilds{}).Execute(dataCtx, nil); err != nil {
		return err
	}
	handler := middlewares.AddDataContext(cfg)(router)

	eng := engine.New(cfg)
	defer eng.Close()

	runnerCtx, stopRunner := context.WithCancel(engine.NewContext(dataCtx, eng))
	runnerDone := make(chan struct{})
	go func() {
		defer close(runnerDone)
		runQueuedBuilds(runnerCtx, cfg)
	}()
	defer func() { stopRunner(); <-runnerDone }() // before the db/engine Close defers

	listenAddr := config.Get(cfg, httpserver.ListenAddrConfig)
	server := &http.Server{Addr: listenAddr, Handler: handler}
	ctrlc.Do(func() { stopRunner(); server.Close() })

	fxlog.Log("listening", fxlog.String("addr", listenAddr))
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Router builds the server's routes on a fresh chi router; Serve listens with it and
// tests drive it directly. Router stays pure routing — DB wiring (connect, migrate,
// data-context middleware) is Serve's, so router tests run without postgres.
func Router(cfg *config.Source) (chi.Router, error) {
	router := chi.NewRouter()
	router.Use(middlewares.Configure(cfg))
	router.Use(middlewares.LogRequests(cfg))

	ctrs := []controllers.Interface{API{}, Auth{}, FluxWebhook{}, Webhooks{}, Setup{}, UI{}}
	for _, ctr := range ctrs {
		if err := ctr.Mount(cfg, router); err != nil {
			return nil, err
		}
	}
	return router, nil
}
