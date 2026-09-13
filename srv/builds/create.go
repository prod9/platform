package builds

import (
	"context"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/validate"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/srv/repos"
)

var _ controllers.Validator = (*Create)(nil)

// Create records the intent to build. The record is the queue — there is no dispatch step
// here — so it carries full intent: which repo, at which ref, resolved to which sha. The
// manual-trigger path accepts owner, repo, ref, module selection, and a retry predecessor;
// source resolution, manifest observation, and attribution are server-owned.
type Create struct {
	Owner   string          `json:"owner"`
	Repo    string          `json:"repo"`
	Ref     string          `json:"ref"`
	Modules ModuleSelection `json:"modules"`
	RetryOf int64           `json:"retry_of"`

	Trigger     Trigger    `json:"-"`
	UserID      int64      `json:"-"`
	CloneURL    string     `json:"-"`
	SHA         string     `json:"-"`
	ManifestRaw string     `json:"-"`
	Manifest    conf.Model `json:"-"`
}

func (c *Create) Validate() error {
	return validate.Multi(
		validate.Required("owner", c.Owner),
		validate.Required("repo", c.Repo),
		validate.Required("ref", c.Ref),
		validate.NonNegative("retry_of", c.RetryOf),
	)
}

func (c *Create) Execute(ctx context.Context, out any) error {
	registration, err := repos.Registered(ctx, c.Owner, c.Repo)
	if err != nil {
		return err
	}
	if c.RetryOf != 0 {
		var matches bool
		if err := data.Get(ctx, &matches, `SELECT EXISTS (SELECT 1 FROM builds
			WHERE id = $1 AND repo_id = $2)`, c.RetryOf, registration.ID); err != nil {
			return err
		}
		if !matches {
			return ErrInvalidRetry
		}
	}

	return data.Run(ctx, func(scope data.Scope) error {
		record := &repos.RecordManifest{RepoID: registration.ID, SHA: c.SHA, Raw: c.ManifestRaw, Manifest: c.Manifest}
		snapshot := &repos.ManifestSnapshot{}
		if err := record.Execute(scope.Context(), snapshot); err != nil {
			return err
		}
		modules, err := repos.ManifestModules(scope.Context(), snapshot.ID)
		if err != nil {
			return err
		}
		selected, err := c.Modules.selectModules(modules)
		if err != nil {
			return err
		}

		var id int64
		err = scope.Get(&id, `INSERT INTO builds
			(trigger, retry_of, user_id, repo_id, manifest_id, clone_url, ref, sha)
			VALUES ($1, NULLIF($2, 0::bigint), $3, $4, $5, $6, $7, $8) RETURNING id`,
			c.Trigger, c.RetryOf, c.UserID, registration.ID, snapshot.ID, c.CloneURL, c.Ref, c.SHA)
		if err != nil {
			return err
		}
		for _, module := range selected {
			if err := scope.Exec(`INSERT INTO build_modules (build_id, manifest_id, manifest_module_id)
				VALUES ($1, $2, $3)`, id, snapshot.ID, module.ID); err != nil {
				return err
			}
		}
		if out == nil {
			return nil
		}
		return scope.Get(out, `SELECT `+buildColumns+` FROM `+buildFrom+` WHERE b.id = $1`, id)
	})
}
