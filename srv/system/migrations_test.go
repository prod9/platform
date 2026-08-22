package system

import (
	"context"
	"errors"
	"testing"

	"fx.prodigy9.co/data/migrator"
	"github.com/stretchr/testify/require"
)

type migrationRunnerStub struct {
	plans   []migrator.Plan
	dirty   bool
	planErr error
	applyAt []string
}

func (s *migrationRunnerStub) Plan(context.Context, migrator.Intent) ([]migrator.Plan, bool, error) {
	return s.plans, s.dirty, s.planErr
}

func (s *migrationRunnerStub) Apply(_ context.Context, plan migrator.Plan) error {
	s.applyAt = append(s.applyAt, plan.Migration.Name)
	return nil
}

func TestProjectMigrationsPreservesOrderedFxPlan(t *testing.T) {
	plans := []migrator.Plan{
		{Action: migrator.ActionMigrate, Migration: migrator.Migration{Name: "202608221000_create_repos"}},
		{Action: migrator.ActionResync, Migration: migrator.Migration{Name: "202608221100_add_owner"}},
		{Action: migrator.ActionPrune, Migration: migrator.Migration{Name: "202608221200_old_table"}},
	}

	require.Equal(t, []MigrationPlan{
		{Action: "migrate", Migration: "202608221000_create_repos"},
		{Action: "update sql", Migration: "202608221100_add_owner"},
		{Action: "remove", Migration: "202608221200_old_table"},
	}, projectMigrations(plans))
}

func TestProjectMigrationsReturnsNonNilEmptyPlan(t *testing.T) {
	require.Equal(t, []MigrationPlan{}, projectMigrations(nil))
}

func TestRunMigrationsRefusesDirtyPlan(t *testing.T) {
	runner := &migrationRunnerStub{
		plans: []migrator.Plan{{
			Action:    migrator.ActionResync,
			Migration: migrator.Migration{Name: "202608221100_changed_sql"},
		}},
		dirty: true,
	}

	err := applyCleanMigrations(t.Context(), runner)

	require.ErrorIs(t, err, errDirtyMigrations)
	require.Empty(t, runner.applyAt)
}

func TestRunMigrationsAppliesCleanPendingPlanInOrder(t *testing.T) {
	runner := &migrationRunnerStub{plans: []migrator.Plan{
		{Action: migrator.ActionMigrate, Migration: migrator.Migration{Name: "202608221000_first"}},
		{Action: migrator.ActionMigrate, Migration: migrator.Migration{Name: "202608221100_second"}},
	}}

	err := applyCleanMigrations(t.Context(), runner)

	require.NoError(t, err)
	require.Equal(t, []string{"202608221000_first", "202608221100_second"}, runner.applyAt)
}

func TestRunMigrationsReturnsPlanningFailure(t *testing.T) {
	planErr := errors.New("plan failed")
	runner := &migrationRunnerStub{planErr: planErr}

	err := applyCleanMigrations(t.Context(), runner)

	require.ErrorIs(t, err, planErr)
	require.Empty(t, runner.applyAt)
}
