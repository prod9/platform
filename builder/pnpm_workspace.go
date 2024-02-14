package builder

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/builder/fileutil"
)

type PNPMWorkspace struct{}

func (PNPMWorkspace) Name() string   { return "pnpm/workspace" }
func (PNPMWorkspace) Layout() Layout { return LayoutWorkspace }
func (PNPMWorkspace) Class() Class   { return ClassInterpreted }

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

	wsdir, err := filepath.Abs(filepath.Join(job.WorkDir, ".."))
	if err != nil {
		return nil, err
	}

	host := sess.Client().Host().Directory(wsdir, dagger.HostDirectoryOpts{
		Exclude: job.Excludes,
	})

	base := BaseImageForJob(sess, job)

	// TODO: Do 2-step builds, install dependencies first, to speed up builds
	builder := withPNPMBuildBase(base)
	builder = withPNPMPkgCache(sess, builder)

	builder = builder.
		WithDirectory("/app", host).
		WithExec([]string{"pnpm", "-r", "install"}).
		WithExec([]string{"pnpm", "-r", "build"})

	outdir := strings.TrimSpace(job.BuildDir)
	if outdir == "" {
		outdir = "build"
	}

	cmd := strings.TrimSpace(job.CommandName)
	if cmd == "" {
		cmd = "/usr/bin/node"
	}

	args := []string{cmd}
	if len(job.CommandArgs) > 0 {
		args = append(args, job.CommandArgs...)
	} else {
		args = append(args, ".")
	}

	runner := withPNPMRunnerBase(base)
	runner = withJobEnv(runner, job)

	runner = runner.
		WithDirectory("/app", builder.Directory("/app/"+job.Name+"/"+outdir))

	runner = withTypeModulePackageJSON(runner).
		WithDefaultArgs(args)
	return runner.Sync(sess.Context())
}
