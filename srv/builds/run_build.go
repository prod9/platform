package builds

import (
	"context"
	"strings"
	"time"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/fxlog"
	"fx.prodigy9.co/worker"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/engine"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/install"
)

// RunBuild carries one build: it prepares the tree, hands it to the engine, and writes what
// the engine reports. The build id is its whole payload — everything else is read from the
// record, which was written complete enough to act on.
//
// 🚨 A build's failure is not this job's failure. A build that failed and was correctly
// recorded is a successful job, so Run returns an error only when the job itself could not
// do its work: the record was unreadable, or the stream was unwritable.
type RunBuild struct {
	BuildID int64 `json:"build_id"`
}

var (
	_ worker.Interface = (*RunBuild)(nil)
	_ worker.Resetter  = (*RunBuild)(nil)
)

func (*RunBuild) Name() string { return "build" }

// Reset clears the payload: fx reuses one instance per job name across every run, so a
// field left standing would be the previous build's.
func (r *RunBuild) Reset() { *r = RunBuild{} }

func (r *RunBuild) Run(ctx context.Context) error {
	build, err := findBuild(ctx, r.BuildID)
	if err != nil {
		return err
	}

	scribe := newTranscriber(ctx, build.ID)
	if err := r.execute(ctx, build, scribe); err != nil {
		// The engine reported nothing, so nothing else will ever mark this build finished.
		// The failure is the build's — a tree that will not clone is not a broken worker —
		// and a stream with no terminal event would be scanned as unclaimed forever.
		scribe.RunDone("", time.Now(), err)
	}

	return scribe.Err()
}

// execute prepares the tree and runs the build on it, returning an error only for the
// failures the engine never got to report. A unit that failed under the engine is already
// in the stream, so those errors are not returned twice.
func (r *RunBuild) execute(ctx context.Context, build *Build, scribe *transcriber) error {
	cacheDir := config.Get(config.FromContext(ctx), CacheDirConfig)
	token, err := cloneToken(ctx)
	if err != nil {
		return err
	}

	prep := &PrepRepo{
		CacheDir: cacheDir,
		CloneURL: build.CloneURL,
		Token:    token,
		Owner:    build.Owner,
		Repo:     build.Repo,
		SHA:      build.SHA,
		BuildID:  build.ID,
	}
	workDir, _, err := prep.Run(ctx)
	if err != nil {
		return err
	}
	defer r.cleanUp(ctx, cacheDir, build)

	cfg, err := conf.Load(workDir)
	if err != nil {
		return err
	}

	sess := engine.NewSession(ctx)
	defer sess.Close()

	// A unit that failed under the engine has already said so in the stream, so only a
	// failure the engine never reported is handed back — an empty plan, or a [modules] the
	// framework layer rejected before a single run opened.
	_, err = sess.BuildAndPublish(ctx, cfg, nil, publishTag(build.Ref), scribe)
	if err != nil && scribe.Silent() {
		return err
	}
	return nil
}

// cleanUp removes the build's worktree. It is best-effort by design: the build is over and
// its outcome is already recorded, so a stale worktree is a cache to sweep rather than a
// reason to call a finished build failed.
func (r *RunBuild) cleanUp(ctx context.Context, cacheDir string, build *Build) {
	remove := &RemoveWorkTree{
		CacheDir: cacheDir,
		Owner:    build.Owner,
		Repo:     build.Repo,
		BuildID:  build.ID,
	}
	if err := remove.Run(ctx); err != nil {
		fxlog.Log("worktree left behind",
			fxlog.Int64("build", build.ID),
			fxlog.String("error", err.Error()))
	}
}

// cloneToken mints a fresh installation token for one repo-prep sync (spec §Repo
// preparation): the ~1h installation identity is exactly what autonomous clones are
// for, and minting per sync keeps anything long-lived off disk.
func cloneToken(ctx context.Context) (string, error) {
	record, err := install.Load(ctx)
	if err != nil {
		return "", err
	}

	client, err := github.NewClient(ctx)
	if err != nil {
		return "", err
	}
	return client.InstallationToken(ctx, record.InstallationID)
}

// publishTag is the tag the images of this build are published under. A ref is a whole ref
// — 'refs/tags/v1.2.3' — and the image carries the version a human pushed.
func publishTag(ref string) string {
	return strings.TrimPrefix(ref, "refs/tags/")
}

func findBuild(ctx context.Context, id int64) (*Build, error) {
	build := &Build{}
	err := data.Get(ctx, build, `SELECT `+buildColumns+` FROM builds WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	return build, nil
}
