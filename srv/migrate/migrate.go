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
	"github.com/jmoiron/sqlx"
)

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

// State reports how many migrations in src are still pending and whether the applied
// set diverges from src (dirty). It is the read half of the migrations install-state
// check — it never mutates the schema.
func State(ctx context.Context, db *sqlx.DB, src migrator.Source) (pending int, dirty bool, err error) {
	plans, dirty, err := migrator.New(db, src).Plan(ctx, migrator.IntentMigrate)
	if err != nil {
		return 0, dirty, err
	}
	return len(plans), dirty, nil
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
