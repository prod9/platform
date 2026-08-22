// Package system owns the installed server's operational state and remediation.
package system

import (
	"context"
	"errors"
	"fmt"

	"fx.prodigy9.co/app"
	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/fxlog"
	"github.com/jmoiron/sqlx"
)

var App = app.Build().Name("system")

// State reports the applied and pending migration counts and whether the database
// diverges from the migrations registered by the composed fx application.
func State(ctx context.Context, db *sqlx.DB) (applied, pending int, dirty bool, err error) {
	source := migrator.FromAuto(config.FromContext(ctx))
	migrations, err := source()
	if err != nil {
		return 0, 0, false, err
	}

	plans, dirty, err := migrator.New(db, source).Plan(ctx, migrator.IntentMigrate)
	if err != nil {
		return 0, 0, dirty, err
	}
	return len(migrations) - len(plans), len(plans), dirty, nil
}

// Run applies every pending registered migration and refuses a dirty schema.
func Run(ctx context.Context, db *sqlx.DB) error {
	source := migrator.FromAuto(config.FromContext(ctx))
	migrationRunner := migrator.New(db, source)

	plans, dirty, err := migrationRunner.Plan(ctx, migrator.IntentMigrate)
	if err != nil {
		return fmt.Errorf("system: migrations: %w", err)
	}
	if dirty {
		return errors.New("system: database diverges from registered migrations")
	}

	for _, plan := range plans {
		if err := migrationRunner.Apply(ctx, plan); err != nil {
			return fmt.Errorf("system: migrate %s: %w", plan.Migration.Name, err)
		}
		fxlog.Log("migrated", fxlog.String("name", plan.Migration.Name))
	}
	return nil
}
