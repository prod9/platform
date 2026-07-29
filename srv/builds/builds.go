// Package builds owns the webhook-triggered build pipeline: the build records that are
// the queue, the event stream they are read through, the GitHub webhook ingest that feeds
// them, per-build repo preparation, and the UI API listing the results. Execution is not
// here — a worker process consumes the records and drives the engine
// (docs/spec/platform-server.md §The worker is a peer process).
package builds

import (
	"context"
	"embed"
	"time"

	"fx.prodigy9.co/data"
)

// Trigger is how a build came to be asked for. Every trigger records the same domain fact
// and differs only in how it was authorized, so the vocabulary is closed here rather than
// growing per caller.
type Trigger string

const (
	TriggerGitHubPush Trigger = "github-push"
	TriggerWebUI      Trigger = "webui"
	TriggerCLI        Trigger = "cli"
	TriggerRetry      Trigger = "retry"
)

// buildColumns reads the record with retry_of flattened to its zero value: a build with no
// parent is the common case, and 0 says that without a pointer at every boundary.
const buildColumns = `
	id, trigger, COALESCE(retry_of, 0) AS retry_of, user_id,
	owner, repo, clone_url, ref, sha, created_at`

// Build is one recorded request to build a repo at a commit — who asked and what for,
// never how it went. It is written once and never updated; how the build went lives in
// its BuildEvent stream.
type Build struct {
	ID        int64     `db:"id"`
	Trigger   Trigger   `db:"trigger"`
	RetryOf   int64     `db:"retry_of"`
	UserID    int64     `db:"user_id"`
	Owner     string    `db:"owner"`
	Repo      string    `db:"repo"`
	CloneURL  string    `db:"clone_url"`
	Ref       string    `db:"ref"`
	SHA       string    `db:"sha"`
	CreatedAt time.Time `db:"created_at"`
}

// Create records the intent to build. The record is the queue — there is no dispatch step
// here — so it carries full intent: which repo, at which ref, resolved to which sha.
type Create struct {
	Trigger  Trigger
	RetryOf  int64
	UserID   int64
	Owner    string
	Repo     string
	CloneURL string
	Ref      string
	SHA      string
}

func (c *Create) Execute(ctx context.Context, out any) error {
	return data.Exec(ctx, `
		INSERT INTO builds (trigger, retry_of, user_id, owner, repo, clone_url, ref, sha)
		VALUES ($1, NULLIF($2, 0::bigint), $3, $4, $5, $6, $7, $8)`,
		c.Trigger, c.RetryOf, c.UserID, c.Owner, c.Repo, c.CloneURL, c.Ref, c.SHA)
}

// Migrations is this fragment's schema; srv aggregates every fragment's SQL at boot.
//
//go:embed *.sql
var Migrations embed.FS
