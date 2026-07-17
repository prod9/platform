// Package srv is the platform server: the API + webhook processor layer above the
// shared build/render/publish packages (docs/spec/platform-server.md). It composes
// the fragment packages (auth, github, builds) into one router, owns the DB
// boot (connect, aggregate fragment migrations, orphan requeue), and serves the
// embedded web UI at / and the API under /api/.
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
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/migrate"
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
	if err := applyMigrations(context.Background(), db); err != nil {
		return err
	}

	// running rows are orphans at boot: single-server model, so a live claimant from a
	// previous process cannot exist.
	dataCtx := data.NewContext(context.Background(), db)
	if err := (&builds.RequeueOrphans{}).Execute(dataCtx, nil); err != nil {
		return err
	}
	handler := middlewares.AddDataContext(cfg)(router)

	eng := engine.New(cfg)
	defer eng.Close()

	runnerCtx, stopRunner := context.WithCancel(engine.NewContext(dataCtx, eng))
	runnerDone := make(chan struct{})
	go func() {
		defer close(runnerDone)
		builds.RunQueued(runnerCtx, cfg)
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
	router.Get("/health", health)

	ctrs := []controllers.Interface{
		auth.SessionCtr{},
		builds.APICtr{},
		builds.WebhookCtr{},
		github.SetupCtr{},
		UI{},
	}
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

// connectDB fails fast: DATABASE_URL must be set and the database reachable before
// the server boots.
func connectDB(cfg *config.Source) (*sqlx.DB, error) {
	if _, ok := config.GetOK(cfg, data.DatabaseURLConfig); !ok {
		return nil, errors.New("srv: DATABASE_URL is required")
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

// fragmentMigrations aggregates every fragment's embedded SQL — the srv-side
// equivalent of fx's Mount collecting fragment migrations.
var fragmentMigrations = migrate.Merged(
	migrator.FromFS(auth.Migrations),
	migrator.FromFS(github.Migrations),
	migrator.FromFS(builds.Migrations),
)

// applyMigrations applies pending embedded migrations non-interactively. Dirty state
// (applied migrations diverging from the embedded SQL) refuses the boot instead of
// silently resyncing — resolving drift is an operator decision.
func applyMigrations(ctx context.Context, db *sqlx.DB) error {
	m := migrator.New(db, fragmentMigrations)

	plans, dirty, err := m.Plan(ctx, migrator.IntentMigrate)
	if err != nil {
		return fmt.Errorf("srv: migrations: %w", err)
	}
	if dirty {
		return errors.New("srv: migrations: db state diverges from embedded migrations")
	}

	for _, plan := range plans {
		if err := m.Apply(ctx, plan); err != nil {
			return fmt.Errorf("srv: migration %s: %w", plan.Migration.Name, err)
		}
		fxlog.Log("migrated", fxlog.String("name", plan.Migration.Name))
	}
	return nil
}
