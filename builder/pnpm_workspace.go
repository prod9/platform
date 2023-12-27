package builder

import (
	"errors"
	"os"
	"path/filepath"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/builder/fileutil"
)

type PNPMWorkspace struct{}

func (PNPMWorkspace) Name() string { return "pnpm/workspace" }
func (PNPMWorkspace) Kind() Kind   { return KindWorkspace }

func (b PNPMWorkspace) Discover(wd string) (map[string]Interface, error) {
	if detected, err := fileutil.DetectFile(wd, "pnpm-workspaces.yaml"); err != nil {
		return nil, err
	} else if !detected {
		return nil, ErrNoBuilder
	}

	// scan for pnpm/basic on subfolders
	// TODO: Could just read the pnpm-workspace.yaml file and parse it as well, have not
	//   spend time to investigate if that is good enough or not yet so duplicating the
	//   logic from go/workspace for now
	mods := map[string]Interface{}
	err := fileutil.WalkSubdirs(wd, func(dir os.DirEntry) error {
		submods, err := PNPMBasic{}.Discover(filepath.Join(wd, dir.Name()))
		if errors.Is(err, ErrNoBuilder) {
			return nil
		}

		// found a pnpm/basic submodule, mark it as using pnpm/workspace
		for submod := range submods {
			mods[submod] = b
		}
		return nil
	})

	if err != nil {
		return nil, err
	} else if len(mods) == 0 {
		return nil, ErrNoBuilder
	} else {
		return mods, nil
	}

}

func (PNPMWorkspace) Build(sess *Session, job *Job) (container *dagger.Container, err error) {
	defer errutil.Wrap("pnpm/workspace", &err)

	host := sess.Client().Host().
		Directory(job.WorkDir, dagger.HostDirectoryOpts{Exclude: job.Excludes})

	base := BaseImageForJob(sess, job).
		WithExec([]string{"apk", "add", "--no-cache", "nodejs-current", "build-base", "python3"}).
		WithExec([]string{"corepack", "enable", "pnpm"}).
		WithDirectory("/app", host)

	// TODO: Do 2-step builds, install dependencies first, to speed up builds
	builder := base.
		WithExec([]string{"pnpm", "i"}).
		WithExec([]string{"pnpm", "-r", "build"})

	runner := builder.
		WithExec([]string{
			"apk", "add", "--no-cache",
			"nodejs-current", "tzdata", "ca-certificates",
		}).
		WithDefaultArgs(dagger.ContainerWithDefaultArgsOpts{
			Args: []string{"/usr/bin/node", filepath.Join(job.PackageName, "build")},
		})

	return runner.Sync(sess.Context())
}
