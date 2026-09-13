// Package builds owns the webhook-triggered build pipeline: the build records that are
// the queue, the event stream they are read through, the GitHub webhook ingest that feeds
// them, module dispatch, and the UI API listing the results. A worker process consumes
// the module records and drives the engine
// (docs/spec/platform-server.md §The worker is a peer process).
package builds

import (
	"context"
	"errors"

	"fx.prodigy9.co/app"
	"fx.prodigy9.co/data"
	"platform.prodigy9.co/srv/install"
)

var (
	ErrInvalidRetry = errors.New("builds: retry predecessor does not belong to this repository")
	App             = app.Build().
			Name("builds").
			EmbedMigrations(Migrations).
			Middlewares(install.ProductGate, install.RecordContext).
			Controllers(BuildCtr{}, WebhookCtr{}).
			Job(&DispatchBuilds{}).
			Job(&BuildModuleJob{})
)

const (
	TriggerGitHubPush Trigger = "github-push"
	TriggerWebUI      Trigger = "webui"
	TriggerCLI        Trigger = "cli"
	TriggerRetry      Trigger = "retry"

	// No predecessor becomes zero at the Go boundary; its database foreign key stays nullable.
	buildColumns = `
	b.id, b.trigger, COALESCE(b.retry_of, 0) AS retry_of, b.user_id,
	b.repo_id, b.manifest_id, r.owner, r.repo, b.clone_url, b.ref, b.sha, b.created_at`

	buildFrom = `builds b JOIN repos r ON r.id = b.repo_id`
)

// Trigger is how a build came to be asked for. Every trigger records the same domain fact
// and differs only in how it was authorized, so the vocabulary is closed here rather than
// growing per caller.
type Trigger string

// Exists reports whether a build row exists. It is the truthful-status lookup for the
// webui's /builds/{id} dynamic route — the server decides the page's status, not the
// browser (spec §The status of a page is the server's answer).
func Exists(ctx context.Context, id int64) (bool, error) {
	var found bool
	err := data.Get(ctx, &found, `SELECT EXISTS (SELECT 1 FROM builds WHERE id = $1)`, id)
	return found, err
}
