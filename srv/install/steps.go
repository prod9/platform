package install

// The four wizard steps. Each check is isolated and install-safe: a missing
// database or schema is a verdict (intervention required / not started), never a
// query sent to fail (docs/spec/installation.md, install-safe checks).

import (
	"context"
	"errors"
	"strings"

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

// Check goes one step past presence: with credentials saved it reads GET /app and
// compares the App's permissions against the required set — saved-but-under-scoped
// is partially ready, and the message names the gap
// (docs/spec/installation.md, the credentials check).
func (appCredentials) Check(ctx context.Context, db *sqlx.DB) (StepState, error) {
	if db == nil {
		return UnknownState, errNoDatabase
	}

	ready, err := settingsSchemaReady(ctx, db)
	if err != nil {
		return UnknownState, err
	}
	if !ready {
		return NotStartedState, nil
	}

	client, err := github.NewClient(data.NewContext(ctx, db))
	if errors.Is(err, github.ErrNoApp) {
		return NotStartedState, nil
	} else if err != nil {
		return UnknownState, err
	}

	perms, err := client.AppPermissions(ctx)
	if err != nil {
		return UnknownState, err
	}
	missing := github.MissingPermissions(perms)
	if len(missing) > 0 {
		return PartiallyReadyState,
			errors.New("app is missing permissions — " + strings.Join(missing, ", "))
	}
	return FullyReadyState, nil
}

type appInstalled struct{}

func (appInstalled) Name() string { return "app-installed" }

func (appInstalled) Check(ctx context.Context, db *sqlx.DB) (StepState, error) {
	return settingsBacked(ctx, db, func(ctx context.Context) error {
		_, err := Load(ctx)
		return err
	}, ErrNotInstalled)
}

// settingsBacked is the shared shape of the settings-reading checks: probe for
// the settings schema first — the probe always parses, so a pre-install server never
// sends a failing statement — then read, folding the reader's absent sentinel into
// not started.
func settingsBacked(ctx context.Context, db *sqlx.DB, read func(context.Context) error, absent error) (StepState, error) {
	if db == nil {
		return UnknownState, errNoDatabase
	}

	ready, err := settingsSchemaReady(ctx, db)
	if err != nil {
		return UnknownState, err
	}
	if !ready {
		return NotStartedState, nil
	}

	err = read(data.NewContext(ctx, db))
	if errors.Is(err, absent) {
		return NotStartedState, nil
	} else if err != nil {
		return UnknownState, err
	}
	return FullyReadyState, nil
}

func settingsSchemaReady(ctx context.Context, db *sqlx.DB) (bool, error) {
	var ready bool
	err := db.GetContext(ctx, &ready, `SELECT to_regclass('public.settings') IS NOT NULL`)
	return ready, err
}
