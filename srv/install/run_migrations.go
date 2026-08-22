package install

import (
	"context"
	"errors"
	"fmt"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/fxlog"
	"github.com/jmoiron/sqlx"
)

// runMigrations applies the complete clean bootstrap plan before the installer writes
// settings-backed state.
func runMigrations(ctx context.Context, db *sqlx.DB) error {
	source := migrator.FromAuto(config.FromContext(ctx))
	runner := migrator.New(db, source)

	plans, dirty, err := runner.Plan(ctx, migrator.IntentMigrate)
	if err != nil {
		return fmt.Errorf("install: migrations: %w", err)
	}
	if dirty {
		return errors.New("install: database diverges from registered migrations")
	}

	for _, plan := range plans {
		if err := runner.Apply(ctx, plan); err != nil {
			return fmt.Errorf("install: migrate %s: %w", plan.Migration.Name, err)
		}
		fxlog.Log("migrated", fxlog.String("name", plan.Migration.Name))
	}
	return nil
}
