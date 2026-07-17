package install

import (
	"context"
	"errors"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
	"github.com/jmoiron/sqlx"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/migrate"
)

// Entry is one install-state check. The webui renders the first non-done entry as the
// next step.
type Entry struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

const (
	StatusDone    = "done"
	StatusPending = "pending"
	StatusError   = "error"
)

// GetState returns the ordered install-state list. db may be nil (no database
// configured); the ctx must carry fx config so app-credentials can be resolved.
func GetState(ctx context.Context, db *sqlx.DB, merged migrator.Source) []Entry {
	return []Entry{
		dbReachable(ctx, db),
		appCredentials(ctx),
		appInstalled(ctx, db),
		migrationsState(ctx, db, merged),
	}
}

// Complete reports whether every entry is done — the "completely installed" conjunction.
func Complete(entries []Entry) bool {
	for _, entry := range entries {
		if entry.Status != StatusDone {
			return false
		}
	}
	return true
}

func dbReachable(ctx context.Context, db *sqlx.DB) Entry {
	if db == nil {
		return Entry{"db-reachable", StatusError, "no database configured"}
	}
	if err := db.PingContext(ctx); err != nil {
		return Entry{"db-reachable", StatusError, err.Error()}
	}
	return Entry{"db-reachable", StatusDone, ""}
}

func appCredentials(ctx context.Context) Entry {
	_, err := github.LoadApp(ctx)
	if errors.Is(err, github.ErrNoApp) {
		return Entry{"app-credentials", StatusError, "app credentials missing from config"}
	} else if err != nil {
		return Entry{"app-credentials", StatusError, err.Error()}
	}
	return Entry{"app-credentials", StatusDone, ""}
}

func appInstalled(ctx context.Context, db *sqlx.DB) Entry {
	if db == nil {
		return Entry{"app-installed", StatusError, "no database configured"}
	}
	if err := db.PingContext(ctx); err != nil {
		return Entry{"app-installed", StatusError, err.Error()}
	}

	_, err := Load(data.NewContext(ctx, db))
	if errors.Is(err, ErrNotInstalled) {
		return Entry{"app-installed", StatusPending, ""}
	} else if err != nil {
		return Entry{"app-installed", StatusError, err.Error()}
	}
	return Entry{"app-installed", StatusDone, ""}
}

func migrationsState(ctx context.Context, db *sqlx.DB, merged migrator.Source) Entry {
	if db == nil {
		return Entry{"migrations", StatusError, "no database configured"}
	}
	if err := db.PingContext(ctx); err != nil {
		return Entry{"migrations", StatusError, err.Error()}
	}

	pending, dirty, err := migrate.State(ctx, db, merged)
	if err != nil {
		return Entry{"migrations", StatusError, err.Error()}
	}
	if dirty {
		return Entry{"migrations", StatusError, "schema diverges from embedded migrations"}
	}
	if pending > 0 {
		return Entry{"migrations", StatusPending, ""}
	}
	return Entry{"migrations", StatusDone, ""}
}
