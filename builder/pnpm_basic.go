package builder

import (
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/builder/fileutil"
)

type PNPMBasic struct{}

func (PNPMBasic) Name() string   { return "pnpm/basic" }
func (PNPMBasic) Layout() Layout { return LayoutBasic }
func (PNPMBasic) Class() Class   { return ClassInterpreted }

func (b PNPMBasic) Discover(wd string) (map[string]Interface, error) {
	if detected, err := fileutil.DetectFile(wd, "pnpm-lock.yaml"); err != nil {
		return nil, err
	} else if !detected {
		return nil, ErrNoBuilder
	}

	name := filepath.Base(wd)
	return map[string]Interface{name: b}, nil
}

func (PNPMBasic) Build(sess *Session, job *Job) (container *dagger.Container, err error) {
	defer errutil.Wrap("pnpm/basic", &err)

	host := sess.Client().Host().
		Directory(job.WorkDir, dagger.HostDirectoryOpts{Exclude: job.Excludes})

	builder := BaseImageForJob(sess, job)
	builder = withPNPMBase(builder)
	builder = withPNPMPkgCache(sess, builder)
	builder = withJobEnv(builder, job)

	builder = builder.
		WithFile("package.json", host.File("package.json")).
		WithFile("pnpm-lock.yaml", host.File("pnpm-lock.yaml")).
		WithExec([]string{"pnpm", "i"}).
		WithDirectory("/app", host).
		WithExec([]string{"pnpm", "build"})

	runner := BaseImageForJob(sess, job)
	runner = withPNPMBase(runner)
	runner = withJobEnv(runner, job)

	runner = runner.
		WithFile("package.json", builder.File("package.json")).
		WithFile("pnpm-lock.yaml", builder.File("pnpm-lock.yaml")).
		WithExec([]string{"pnpm", "i"})

	outdir := strings.TrimSpace(job.BuildDir)
	if outdir == "" {
		outdir = "build"
	}

	cmd := strings.TrimSpace(job.CommandName)
	if cmd == "" {
		cmd = "/usr/local/bin/node"
	}

	args := []string{cmd}
	if len(job.CommandArgs) > 0 {
		args = append(args, job.CommandArgs...)
	} else {
		args = append(args, ".")
	}

	runner = runner.
		WithDirectory("/app", builder.Directory(outdir)).
		WithDefaultArgs(args)
	return runner, nil
}
