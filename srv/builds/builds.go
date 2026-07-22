// Package builds owns the webhook-triggered build pipeline: the builds queue, the
// GitHub webhook ingest that feeds it, per-build repo preparation, and the UI API
// listing the results. Execution is not here — a worker peer of srv reads the queue
// and drives the engine (docs/spec/platform-server.md §The worker is a peer of srv).
package builds

import (
	"context"
	"embed"
	"time"

	"fx.prodigy9.co/data"
)

// Build is the record of one webhook-triggered build, mapping the builds table:
// queued by Create, advanced by the worker.
type Build struct {
	ID        int64     `db:"id"`
	Owner     string    `db:"owner"`
	Repo      string    `db:"repo"`
	CloneURL  string    `db:"clone_url"`
	Tag       string    `db:"tag"`
	SHA       string    `db:"sha"`
	Status    string    `db:"status"`
	Error     string    `db:"error"`
	Image     string    `db:"image"`
	Digest    string    `db:"digest"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// Create records a queued build row for a pushed version tag.
type Create struct {
	Owner    string
	Repo     string
	CloneURL string
	Tag      string
	SHA      string
}

func (c *Create) Execute(ctx context.Context, out any) error {
	return data.Exec(ctx, `
		INSERT INTO builds (owner, repo, clone_url, tag, sha)
		VALUES ($1, $2, $3, $4, $5)`,
		c.Owner, c.Repo, c.CloneURL, c.Tag, c.SHA)
}

// Migrations is this fragment's schema; srv aggregates every fragment's SQL at boot.
//
//go:embed *.sql
var Migrations embed.FS
