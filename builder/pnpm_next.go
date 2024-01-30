package builder

import (
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/builder/fileutil"
)

type PNPMNext struct{}

func (PNPMNext) Name() string   { return "pnpm/next" }
func (PNPMNext) Layout() Layout { return LayoutBasic }
func (PNPMNext) Class() Class   { return ClassInterpreted }

func (b PNPMNext) Discover(wd string) (map[string]Interface, error) {
	if detected, err := fileutil.DetectFile(wd, "pnpm-lock.yaml"); err != nil {
		return nil, err
	} else if !detected {
		return nil, ErrNoBuilder
	}

	name := filepath.Base(wd)
	return map[string]Interface{name: b}, nil
}

func (PNPMNext) Build(sess *Session, job *Job) (container *dagger.Container, err error) {
	defer errutil.Wrap("pnpm/next", &err)

	host := sess.Client().Host().
		Directory(job.WorkDir, dagger.HostDirectoryOpts{Exclude: job.Excludes})

	builder := BaseImageForJob(sess, job)
	builder = withPNPMBuildBase(builder)
	builder = withPNPMPkgCache(sess, builder)

	builder = builder.
		WithFile("package.json", host.File("package.json")).
		WithFile("pnpm-lock.yaml", host.File("pnpm-lock.yaml")).
		WithExec([]string{"pnpm", "i"}).
		WithDirectory("/app", host).
		WithExec([]string{"pnpm", "build"})

	runner := BaseImageForJob(sess, job)
	runner = withPNPMRunnerBase(runner)
	runner = withJobEnv(runner, job)

	runner = runner.
		WithFile("package.json", builder.File("package.json")).
		WithFile("pnpm-lock.yaml", builder.File("pnpm-lock.yaml")).
		WithExec([]string{"pnpm", "i"})

	defaultCmd := "pnpm"
	cmd := strings.TrimSpace(job.CommandName)
	if cmd == "" {
		cmd = defaultCmd
	}

	args := []string{cmd}
	if len(job.CommandArgs) > 0 {
		args = append(args, job.CommandArgs...)
	} else if cmd == defaultCmd {
		args = append(args, ".")
	}

	runner = runner.
		WithDirectory("/app", host).
		WithDefaultArgs(dagger.ContainerWithDefaultArgsOpts{Args: args})
	return runner, nil
}
