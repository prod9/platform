package repos

import (
	"context"
	"encoding/json"
	"maps"
	"slices"

	"fx.prodigy9.co/data"
	"platform.prodigy9.co/conf"
)

// RecordManifest preserves the first complete observation of a repository commit.
// Its caller's scope includes registration or build creation in the same transaction.
type RecordManifest struct {
	RepoID   int64
	SHA      string
	Raw      string
	Manifest conf.Model
}

func (r *RecordManifest) Execute(ctx context.Context, out any) error {
	return data.Run(ctx, func(scope data.Scope) error {
		id, err := insertManifestSnapshot(scope, r.RepoID, r.SHA, r.Raw, r.Manifest)
		if err != nil && !data.IsNoRows(err) {
			return err
		}
		if err == nil {
			if err := r.insertModules(scope, id); err != nil {
				return err
			}
		}
		return scope.Get(out, `SELECT id, repo_id, sha FROM repo_manifests
			WHERE repo_id = $1 AND sha = $2`, r.RepoID, r.SHA)
	})
}

func (r *RecordManifest) insertModules(scope data.Scope, manifestID int64) error {
	for _, name := range slices.Sorted(maps.Keys(r.Manifest.Modules)) {
		if err := insertManifestModule(scope, manifestID, name, *r.Manifest.Modules[name]); err != nil {
			return err
		}
	}
	return nil
}

func insertManifestSnapshot(scope data.Scope, repoID int64, sha, raw string, model conf.Model) (int64, error) {
	excludes := model.Excludes
	if excludes == nil {
		excludes = []string{}
	}
	varsMap := model.Vars
	if varsMap == nil {
		varsMap = map[string]any{}
	}
	vars, err := json.Marshal(varsMap)
	if err != nil {
		return 0, err
	}

	var snapshotID int64
	err = scope.Get(&snapshotID, `
			INSERT INTO repo_manifests (
				repo_id, sha, raw, maintainer, repository, platform,
				local_arch, publish_arch, strategy, publish_policy, excludes, vars
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
				ON CONFLICT (repo_id, sha) DO NOTHING
				RETURNING id`,
		repoID, sha, raw, model.Maintainer, model.Repository, model.Platform,
		model.LocalArch, model.PublishArch, model.Strategy, model.Server.Publish,
		excludes, vars)
	if err != nil {
		return 0, err
	}
	return snapshotID, nil
}

func insertManifestModule(scope data.Scope, manifestID int64, name string, module conf.Module) error {
	envMap := module.Env
	if envMap == nil {
		envMap = map[string]string{}
	}
	commandArgs := module.CommandArgs
	if commandArgs == nil {
		commandArgs = []string{}
	}
	assetDirs := module.AssetDirs
	if assetDirs == nil {
		assetDirs = []string{}
	}
	env, err := json.Marshal(envMap)
	if err != nil {
		return err
	}

	return scope.Exec(`
		INSERT INTO repo_manifest_modules (
			manifest_id, name, workdir, timeout_ns, framework, env, port,
			command_name, command_args, asset_dirs, build_dir, go_version,
			image_name, package_name
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		manifestID, name, module.WorkDir, int64(module.Timeout), module.Framework, env,
		module.Port, module.CommandName, commandArgs, assetDirs,
		module.BuildDir, module.GoVersion, module.ImageName, module.PackageName)
}
