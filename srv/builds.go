package srv

import (
	"context"
	"errors"
	"time"

	"fx.prodigy9.co/data"
)

var ErrNoQueuedBuild = errors.New("srv: no queued build")

// Build is the srv-owned record of one webhook-triggered build, mapping the builds
// table: queued by CreateBuild, claimed by ClaimBuild, finished by FinishBuild or
// FailBuild.
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

// CreateBuild records a queued build row for a pushed version tag.
type CreateBuild struct {
	Owner    string
	Repo     string
	CloneURL string
	Tag      string
	SHA      string
}

func (c *CreateBuild) Execute(ctx context.Context, out any) error {
	return data.Exec(ctx, `
		INSERT INTO builds (owner, repo, clone_url, tag, sha)
		VALUES ($1, $2, $3, $4, $5)`,
		c.Owner, c.Repo, c.CloneURL, c.Tag, c.SHA)
}

// ClaimBuild atomically claims the oldest queued build, flipping it to running and
// filling out (a *Build) with the claimed row. SKIP LOCKED keeps concurrent claimants
// from blocking on (or double-claiming) the same row; an empty queue is
// ErrNoQueuedBuild.
type ClaimBuild struct{}

func (c *ClaimBuild) Execute(ctx context.Context, out any) error {
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
		return ErrNoQueuedBuild
	}
	return err
}

// RequeueOrphanBuilds flips every running build back to queued. Boot-time recovery:
// in the single-server model, any row still running when the process starts belonged
// to a crashed or killed predecessor — an orphan by definition.
type RequeueOrphanBuilds struct{}

func (r *RequeueOrphanBuilds) Execute(ctx context.Context, out any) error {
	return data.Exec(ctx, `
		UPDATE builds
		SET status = 'queued', updated_at = now()
		WHERE status = 'running'`)
}

// FinishBuild marks a claimed build succeeded, recording what it published.
type FinishBuild struct {
	ID     int64
	Image  string
	Digest string
}

func (f *FinishBuild) Execute(ctx context.Context, out any) error {
	return data.Exec(ctx, `
		UPDATE builds
		SET status = 'succeeded', image = $2, digest = $3, updated_at = now()
		WHERE id = $1`,
		f.ID, f.Image, f.Digest)
}

// FailBuild marks a claimed build failed, recording the error that stopped it.
type FailBuild struct {
	ID    int64
	Error string
}

func (f *FailBuild) Execute(ctx context.Context, out any) error {
	return data.Exec(ctx, `
		UPDATE builds
		SET status = 'failed', error = $2, updated_at = now()
		WHERE id = $1`,
		f.ID, f.Error)
}
