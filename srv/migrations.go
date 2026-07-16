package srv

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/fxlog"
	"github.com/jmoiron/sqlx"
)

//go:embed *.sql
var migrations embed.FS

// connectDB fails fast: DATABASE_URL must be set and the database reachable before the
// server boots.
func connectDB(cfg *config.Source) (*sqlx.DB, error) {
	if _, ok := config.GetOK(cfg, data.DatabaseURLConfig); !ok {
		return nil, errors.New("srv: DATABASE_URL is required")
	}

	db, err := data.Connect(cfg)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("srv: database unreachable: %w", err)
	}
	return db, nil
}

// migrate applies pending embedded migrations non-interactively. Dirty state (applied
// migrations diverging from the embedded SQL) refuses the boot instead of silently
// resyncing — resolving drift is an operator decision.
func migrate(ctx context.Context, db *sqlx.DB) error {
	m := migrator.New(db, migrator.FromFS(migrations))

	plans, dirty, err := m.Plan(ctx, migrator.IntentMigrate)
	if err != nil {
		return fmt.Errorf("srv: migrations: %w", err)
	}
	if dirty {
		return errors.New("srv: migrations: db state diverges from embedded migrations")
	}

	for _, plan := range plans {
		if err := m.Apply(ctx, plan); err != nil {
			return fmt.Errorf("srv: migration %s: %w", plan.Migration.Name, err)
		}
		fxlog.Log("migrated", fxlog.String("name", plan.Migration.Name))
	}
	return nil
}
