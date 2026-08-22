package system

import (
	"context"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
)

type MigrationPlan struct {
	Action    string `json:"action"`
	Migration string `json:"migration"`
}

func Migrations(ctx context.Context) ([]MigrationPlan, error) {
	source := migrator.FromAuto(config.FromContext(ctx))
	plans, _, err := migrator.New(data.FromContext(ctx), source).Plan(ctx, migrator.IntentMigrate)
	if err != nil {
		return nil, err
	}

	return projectMigrations(plans), nil
}

func projectMigrations(plans []migrator.Plan) []MigrationPlan {
	projected := make([]MigrationPlan, 0, len(plans))
	for _, plan := range plans {
		projected = append(projected, MigrationPlan{
			Action:    plan.Action.String(),
			Migration: plan.Migration.Name,
		})
	}
	return projected
}
