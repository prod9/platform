// Package migrate composes fragment migration sources for the srv layer and runs
// them. It is a leaf: the installer fragment checks and applies migrations through
// State/Run without importing srv, so there is no srv→install→srv cycle.
package migrate

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/fxlog"
	"fx.prodigy9.co/worker"
	"github.com/jmoiron/sqlx"
)

// JobsTable is fx's own jobs schema, applied with ours rather than left to the worker.
// worker.Start() creates the table itself, but only once the process is already running —
// too late for that process to have seeded its first job. Applying it here means a migrated
// database is ready for work before any worker starts. The SQL is fx's, never a copy.
var JobsTable = migrator.FromSQL("202607300000_create_jobs",
	worker.CreateJobsTableSQL,
	"DROP TABLE jobs;")

// Merged combines migration sources into one, re-sorted by name so timestamps
// interleave across fragments exactly as they would in a single directory.
func Merged(sources ...migrator.Source) migrator.Source {
	return func() ([]migrator.Migration, error) {
		all := []migrator.Migration{}
		for _, source := range sources {
			migrations, err := source()
			if err != nil {
				return nil, err
			}
			all = append(all, migrations...)
		}

		slices.SortFunc(all, func(a, b migrator.Migration) int {
			return strings.Compare(a.Name, b.Name)
		})
		return all, nil
	}
}

// State reports how many migrations in src are applied and still pending, and whether
// the applied set diverges from src (dirty). It is the read half of the migrations
// install-state check — it never mutates the schema.
func State(ctx context.Context, db *sqlx.DB, src migrator.Source) (applied, pending int, dirty bool, err error) {
	migrations, err := src()
	if err != nil {
		return 0, 0, false, err
	}

	plans, dirty, err := migrator.New(db, src).Plan(ctx, migrator.IntentMigrate)
	if err != nil {
		return 0, 0, dirty, err
	}
	return len(migrations) - len(plans), len(plans), dirty, nil
}

// Run applies every pending migration in src, refusing a dirty schema rather than
// silently resyncing — resolving drift is an operator decision.
func Run(ctx context.Context, db *sqlx.DB, src migrator.Source) error {
	m := migrator.New(db, src)

	plans, dirty, err := m.Plan(ctx, migrator.IntentMigrate)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if dirty {
		return errors.New("migrate: db state diverges from embedded migrations")
	}

	for _, plan := range plans {
		if err := m.Apply(ctx, plan); err != nil {
			return fmt.Errorf("migrate: %s: %w", plan.Migration.Name, err)
		}
		fxlog.Log("migrated", fxlog.String("name", plan.Migration.Name))
	}
	return nil
}
