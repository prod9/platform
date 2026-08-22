package install

import (
	"context"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data/migrator"
	"github.com/jmoiron/sqlx"
)

// migrationState reports the installer's bootstrap progress against every migration
// registered by the composed fx application.
func migrationState(ctx context.Context, db *sqlx.DB) (applied, pending int, dirty bool, err error) {
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
