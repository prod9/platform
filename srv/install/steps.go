package install

// The four wizard steps. Each check is isolated and install-safe: a missing
// database or schema is a verdict (intervention required / not started), never a
// query sent to fail (docs/spec/installation.md, install-safe checks).

import (
	"context"
	"errors"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
	"github.com/jmoiron/sqlx"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/migrate"
)

var errNoDatabase = errors.New("no database configured")

type dbReachable struct{}

func (dbReachable) Name() string { return "db-reachable" }

func (dbReachable) Check(ctx context.Context, db *sqlx.DB) (StepState, error) {
	if db == nil {
		return InterventionRequiredState, errNoDatabase
	}
	if err := db.PingContext(ctx); err != nil {
		return InterventionRequiredState, err
	}
	return FullyReadyState, nil
}

type migrations struct{ src migrator.Source }

func (migrations) Name() string { return "migrations" }

func (m migrations) Check(ctx context.Context, db *sqlx.DB) (StepState, error) {
	if db == nil {
		return UnknownState, errNoDatabase
	}

	applied, pending, dirty, err := migrate.State(ctx, db, m.src)
	if err != nil {
		return UnknownState, err
	}
	if dirty {
		return InterventionRequiredState, errors.New("schema diverges from embedded migrations")
	}

	switch {
	case pending == 0:
		return FullyReadyState, nil
	case applied == 0:
		return NotStartedState, nil
	default:
		return PartiallyReadyState, nil
	}
}

type appCredentials struct{}

func (appCredentials) Name() string { return "app-credentials" }

func (appCredentials) Check(ctx context.Context, db *sqlx.DB) (StepState, error) {
	return settingsBacked(ctx, db, func(ctx context.Context) error {
		_, err := github.LoadApp(ctx)
		return err
	}, github.ErrNoApp)
}

type appInstalled struct{}

func (appInstalled) Name() string { return "app-installed" }

func (appInstalled) Check(ctx context.Context, db *sqlx.DB) (StepState, error) {
	return settingsBacked(ctx, db, func(ctx context.Context) error {
		_, err := Load(ctx)
		return err
	}, ErrNotInstalled)
}

// settingsBacked is the shared shape of the two settings-reading checks: probe for
// the settings schema first — the probe always parses, so a pre-install server never
// sends a failing statement — then read, folding the reader's absent sentinel into
// not started.
func settingsBacked(ctx context.Context, db *sqlx.DB, read func(context.Context) error, absent error) (StepState, error) {
	if db == nil {
		return UnknownState, errNoDatabase
	}

	var ready bool
	if err := db.GetContext(ctx, &ready, `SELECT to_regclass('public.settings') IS NOT NULL`); err != nil {
		return UnknownState, err
	}
	if !ready {
		return NotStartedState, nil
	}

	err := read(data.NewContext(ctx, db))
	if errors.Is(err, absent) {
		return NotStartedState, nil
	} else if err != nil {
		return UnknownState, err
	}
	return FullyReadyState, nil
}
