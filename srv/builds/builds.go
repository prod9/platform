// Package builds owns the webhook-triggered build pipeline: the build records that are
// the queue, the event stream they are read through, the GitHub webhook ingest that feeds
// them, per-build repo preparation, and the UI API listing the results. Execution is not
// here — a worker process consumes the records and drives the engine
// (docs/spec/platform-server.md §The worker is a peer process).
package builds

import (
	"context"
	"time"

	"fx.prodigy9.co/app"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/validate"
	"platform.prodigy9.co/srv/install"
)

var App = app.Build().
	Name("builds").
	EmbedMigrations(Migrations).
	Middlewares(install.ProductGate, install.RecordContext).
	Controllers(BuildCtr{}, WebhookCtr{}).
	Job(&ScanBuilds{}).
	Job(&RunBuild{})

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
// here — so it carries full intent: which repo, at which ref, resolved to which sha. The
// manual-trigger path reads owner, repo, and ref from the request; every other field is
// derived server-side by the caller.
type Create struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Ref   string `json:"ref"`

	Trigger  Trigger `json:"-"`
	RetryOf  int64   `json:"-"`
	UserID   int64   `json:"-"`
	CloneURL string  `json:"-"`
	SHA      string  `json:"-"`
}

var _ controllers.Validator = (*Create)(nil)

func (c *Create) Validate() error {
	return validate.Multi(
		validate.Required("owner", c.Owner),
		validate.Required("repo", c.Repo),
		validate.Required("ref", c.Ref),
	)
}

func (c *Create) Execute(ctx context.Context, out any) error {
	const insert = `
		INSERT INTO builds (trigger, retry_of, user_id, owner, repo, clone_url, ref, sha)
		VALUES ($1, NULLIF($2, 0::bigint), $3, $4, $5, $6, $7, $8)`

	if out == nil {
		return data.Exec(ctx, insert,
			c.Trigger, c.RetryOf, c.UserID, c.Owner, c.Repo, c.CloneURL, c.Ref, c.SHA)
	}
	return data.Get(ctx, out, insert+` RETURNING `+buildColumns,
		c.Trigger, c.RetryOf, c.UserID, c.Owner, c.Repo, c.CloneURL, c.Ref, c.SHA)
}

// Exists reports whether a build row exists. It is the truthful-status lookup for the
// webui's /builds/{id} dynamic route — the server decides the page's status, not the
// browser (spec §The status of a page is the server's answer).
func Exists(ctx context.Context, id int64) (bool, error) {
	var found bool
	err := data.Get(ctx, &found, `SELECT EXISTS (SELECT 1 FROM builds WHERE id = $1)`, id)
	return found, err
}
