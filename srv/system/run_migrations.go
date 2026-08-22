package system

import (
	"context"
	"errors"
	"fmt"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/fxlog"
	"fx.prodigy9.co/httpserver/controllers"
)

var errDirtyMigrations = errors.New("system: database diverges from registered migrations")

type RunMigrations struct{}

var _ controllers.Action = (*RunMigrations)(nil)

type migrationRunner interface {
	Plan(context.Context, migrator.Intent) ([]migrator.Plan, bool, error)
	Apply(context.Context, migrator.Plan) error
}

func (*RunMigrations) Execute(ctx context.Context, _ any) error {
	source := migrator.FromAuto(config.FromContext(ctx))
	runner := migrator.New(data.FromContext(ctx), source)
	return applyCleanMigrations(ctx, runner)
}

func applyCleanMigrations(ctx context.Context, runner migrationRunner) error {
	plans, dirty, err := runner.Plan(ctx, migrator.IntentMigrate)
	if err != nil {
		return fmt.Errorf("system: migrations: %w", err)
	}
	if dirty {
		return errDirtyMigrations
	}

	for _, plan := range plans {
		if err := runner.Apply(ctx, plan); err != nil {
			return fmt.Errorf("system: migrate %s: %w", plan.Migration.Name, err)
		}
		fxlog.Log("migrated", fxlog.String("name", plan.Migration.Name))
	}
	return nil
}
