// Package repos records repo registration — *this repo is onboarded to build here* —
// and nothing else. Visibility stays gated live per request: the API intersects the
// registered set with what the session user can currently reach on GitHub, so no
// permission state is ever stored (docs/spec/platform-server.md §Repos are registered,
// visibility is live).
package repos

import (
	"context"
	"time"

	"fx.prodigy9.co/app"
	"fx.prodigy9.co/data"
	"platform.prodigy9.co/srv/install"
)

var App = app.Build().
	Name("repos").
	EmbedMigrations(Migrations).
	Middlewares(install.ProductGate, install.RecordContext).
	Controllers(RepoCtr{})

// Repo is one registration row.
type Repo struct {
	ID           int64     `db:"id"`
	Owner        string    `db:"owner"`
	Repo         string    `db:"repo"`
	RegisteredBy int64     `db:"registered_by"`
	CreatedAt    time.Time `db:"created_at"`
}

// ListRegistered reads every registration, oldest first — the stored half of the
// registered∩live visibility rule.
func ListRegistered(ctx context.Context) ([]*Repo, error) {
	repos := []*Repo{}
	err := data.Select(ctx, &repos, `SELECT * FROM repos ORDER BY id`)
	return repos, err
}
