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
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
}

// Status is the verdict of one check. The three values below are the whole domain — a
// bare string cannot stand in for one.
type Status string

const (
	StatusDone    Status = "done"
	StatusPending Status = "pending"
	StatusError   Status = "error"
)

// GetState returns the ordered install-state list; db may be nil (no database
// configured). The order is the wizard's: every install-time value lives in settings,
// and the settings table exists only once migrations ran, so migrations precede both
// settings-backed entries and the claim stays last (docs/spec/installation.md).
func GetState(ctx context.Context, db *sqlx.DB, merged migrator.Source) []Entry {
	reach := dbReachable(ctx, db)
	if reach.Status != StatusDone {
		return []Entry{
			reach,
			{"migrations", StatusError, reach.Message},
			{"app-credentials", StatusError, reach.Message},
			{"app-installed", StatusError, reach.Message},
		}
	}

	return []Entry{
		reach,
		migrationsState(ctx, db, merged),
		appCredentials(ctx, db),
		appInstalled(ctx, db),
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

// appCredentials, appInstalled, and migrationsState assume a reachable db — GetState
// establishes that once via dbReachable and never calls them otherwise.
func appCredentials(ctx context.Context, db *sqlx.DB) Entry {
	_, err := github.LoadApp(data.NewContext(ctx, db))
	if errors.Is(err, github.ErrNoApp) {
		return Entry{"app-credentials", StatusPending, ""}
	} else if err != nil {
		return Entry{"app-credentials", StatusError, err.Error()}
	}
	return Entry{"app-credentials", StatusDone, ""}
}

func appInstalled(ctx context.Context, db *sqlx.DB) Entry {
	_, err := Load(data.NewContext(ctx, db))
	if errors.Is(err, ErrNotInstalled) {
		return Entry{"app-installed", StatusPending, ""}
	} else if err != nil {
		return Entry{"app-installed", StatusError, err.Error()}
	}
	return Entry{"app-installed", StatusDone, ""}
}

func migrationsState(ctx context.Context, db *sqlx.DB, merged migrator.Source) Entry {
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
