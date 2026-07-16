package srv

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/fxlog"
	"fx.prodigy9.co/httpserver/controllers"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/engine"
)

// buildPollInterval paces the empty-queue poll; a claimed run re-claims immediately
// after finishing, so bursts drain without waiting on the tick.
const buildPollInterval = 2 * time.Second

// publishBuild seams the engine-facing half of a run (the loadGitHubApp pattern) so the
// loop tests without dagger.
var publishBuild = runBuild

// runQueuedBuilds consumes queued builds until ctx is canceled: claim the oldest queued
// row, run it, record the outcome. It is the server half of the one-publish-engine
// model — the same BuildAndPublish the local publish command drives.
func runQueuedBuilds(ctx context.Context, cfg *config.Source) {
	for ctx.Err() == nil {
		build, err := ClaimBuild(ctx)
		if errors.Is(err, ErrNoQueuedBuild) || errors.Is(err, context.Canceled) {
			waitPollTick(ctx)
			continue
		} else if err != nil {
			fxlog.Error(err)
			waitPollTick(ctx)
			continue
		}

		fxlog.Log("build claimed", buildAttrs(build)...)
		image, digest, err := publishBuild(ctx, cfg, build)
		if err != nil {
			recordOutcome(ctx, &FailBuild{ID: build.ID, Error: err.Error()})
			fxlog.Log("build failed",
				append(buildAttrs(build), fxlog.String("error", err.Error()))...)
			continue
		}

		recordOutcome(ctx, &FinishBuild{ID: build.ID, Image: image, Digest: digest})
		fxlog.Log("build succeeded",
			append(buildAttrs(build), fxlog.String("image", image))...)
	}
}

// runBuild is one build end-to-end: prep the worktree, load the project's
// platform.toml, and drive the shared BuildAndPublish under the build's tag. A
// multi-module repo publishes several images; names and digests join
// newline-separated into the record's two text columns.
func runBuild(ctx context.Context, cfg *config.Source, build *Build) (image string, digest string, err error) {
	prep := &PrepRepo{
		CacheDir: config.Get(cfg, CacheDirConfig),
		CloneURL: build.CloneURL,
		Owner:    build.Owner,
		Repo:     build.Repo,
		SHA:      build.SHA,
		BuildID:  build.ID,
	}
	workDir, _, err := prep.Run(ctx)
	if err != nil {
		return "", "", err
	}
	defer removeWorkTree(ctx, prep)

	model, err := conf.Load(workDir)
	if err != nil {
		return "", "", err
	}
	results, err := engine.BuildAndPublish(ctx, model, nil, build.Tag)
	if err != nil {
		return "", "", err
	}

	images, digests := make([]string, len(results)), make([]string, len(results))
	for i, result := range results {
		images[i], digests[i] = result.ImageName, result.ImageHash
	}
	return strings.Join(images, "\n"), strings.Join(digests, "\n"), nil
}

// removeWorkTree cleans up after a build, surviving a canceled ctx so shutdown still
// cleans. A failure only leaks a cache-dir worktree, so it logs instead of failing an
// otherwise-finished build.
func removeWorkTree(ctx context.Context, prep *PrepRepo) {
	remove := &RemoveWorkTree{
		CacheDir: prep.CacheDir,
		Owner:    prep.Owner,
		Repo:     prep.Repo,
		BuildID:  prep.BuildID,
	}
	if err := remove.Run(context.WithoutCancel(ctx)); err != nil {
		fxlog.Error(err)
	}
}

// recordOutcome writes a finish/fail mutation; a write failure leaves the row running,
// which only costs an operator a stale status — never the loop.
func recordOutcome(ctx context.Context, outcome controllers.Action) {
	if err := outcome.Execute(ctx, nil); err != nil {
		fxlog.Error(err)
	}
}

func waitPollTick(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-time.After(buildPollInterval):
	}
}

func buildAttrs(build *Build) []slog.Attr {
	return []slog.Attr{
		fxlog.Int64("id", build.ID),
		fxlog.String("repo", build.Owner+"/"+build.Repo),
		fxlog.String("tag", build.Tag),
	}
}
