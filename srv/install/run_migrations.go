package install

import (
	"context"
	"errors"
	"fmt"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/fxlog"
)

// RunMigrations applies the complete clean bootstrap plan before the installer writes
// settings-backed state.
type RunMigrations struct{}

func (*RunMigrations) Execute(ctx context.Context, out any) error {
	source := migrator.FromAuto(config.FromContext(ctx))
	runner := migrator.New(data.FromContext(ctx), source)

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
