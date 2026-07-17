// Package builds owns the webhook-triggered build pipeline: the builds queue, the
// GitHub webhook ingest that feeds it, per-build repo preparation, the runner loop
// that drives the shared publish engine, and the UI API listing the results.
package builds

import (
	"context"
	"errors"
	"time"

	"fx.prodigy9.co/data"
)

var ErrNoneQueued = errors.New("builds: no queued build")

// Build is the record of one webhook-triggered build, mapping the builds table:
// queued by Create, claimed by Claim, finished by Finish or Fail.
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

// Claim atomically claims the oldest queued build, flipping it to running and
// filling out (a *Build) with the claimed row. SKIP LOCKED keeps concurrent
// claimants from blocking on (or double-claiming) the same row; an empty queue is
// ErrNoneQueued.
type Claim struct{}

func (c *Claim) Execute(ctx context.Context, out any) error {
	err := data.Get(ctx, out, `
		UPDATE builds
		SET status = 'running', updated_at = now()
		WHERE id = (
			SELECT id FROM builds
			WHERE status = 'queued'
			ORDER BY id
			LIMIT 1
			FOR UPDATE SKIP LOCKED)
		RETURNING *`)
	if data.IsNoRows(err) {
		return ErrNoneQueued
	}
	return err
}

// RequeueOrphans flips every running build back to queued. Boot-time recovery: in
// the single-server model, any row still running when the process starts belonged
// to a crashed or killed predecessor — an orphan by definition.
type RequeueOrphans struct{}

func (r *RequeueOrphans) Execute(ctx context.Context, out any) error {
	return data.Exec(ctx, `
		UPDATE builds
		SET status = 'queued', updated_at = now()
		WHERE status = 'running'`)
}

// Finish marks a claimed build succeeded, recording what it published.
type Finish struct {
	ID     int64
	Image  string
	Digest string
}

func (f *Finish) Execute(ctx context.Context, out any) error {
	return data.Exec(ctx, `
		UPDATE builds
		SET status = 'succeeded', image = $2, digest = $3, updated_at = now()
		WHERE id = $1`,
		f.ID, f.Image, f.Digest)
}

// Fail marks a claimed build failed, recording the error that stopped it.
type Fail struct {
	ID    int64
	Error string
}

func (f *Fail) Execute(ctx context.Context, out any) error {
	return data.Exec(ctx, `
		UPDATE builds
		SET status = 'failed', error = $2, updated_at = now()
		WHERE id = $1`,
		f.ID, f.Error)
}
