// Package srv is the platform server: the API + webhook processor layer above the
// shared build/render/publish packages (docs/spec/platform-server.md). It serves the
// embedded web UI at / and the API under /api/.
package srv

import (
	"context"
	"errors"
	"net/http"
	"time"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/ctrlc"
	"fx.prodigy9.co/fxlog"
	"fx.prodigy9.co/httpserver"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/httpserver/middlewares"
	"fx.prodigy9.co/httpserver/render"
	"github.com/go-chi/chi/v5"
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
	if err := migrate(context.Background(), db); err != nil {
		return err
	}
	if err := db.Close(); err != nil { // boot pool done; AddDataContext owns the serving pool
		return err
	}
	handler := middlewares.AddDataContext(cfg)(router)

	listenAddr := config.Get(cfg, httpserver.ListenAddrConfig)
	server := &http.Server{Addr: listenAddr, Handler: handler}
	ctrlc.Do(func() { server.Close() })

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

	ctrs := []controllers.Interface{API{}, UI{}}
	for _, ctr := range ctrs {
		if err := ctr.Mount(cfg, router); err != nil {
			return nil, err
		}
	}
	return router, nil
}

// API serves the platform API under /api/.
type API struct{}

var _ controllers.Interface = API{}

func (API) Mount(cfg *config.Source, router chi.Router) error {
	router.Get("/api/health", health)
	return nil
}

func health(resp http.ResponseWriter, req *http.Request) {
	render.JSON(resp, req, struct {
		Time time.Time `json:"time"`
	}{time.Now()})
}
