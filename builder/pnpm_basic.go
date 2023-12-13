package builder

import (
	"context"
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/builder/fileutil"
)

type PNPMBasic struct{}

func (PNPMBasic) Name() string { return "pnpm/basic" }
func (PNPMBasic) Kind() Kind   { return KindBasic }

func (b PNPMBasic) Discover(wd string) (map[string]Interface, error) {
	if detected, err := fileutil.DetectFile(wd, "pnpm-lock.yaml"); err != nil {
		return nil, err
	} else if !detected {
		return nil, ErrNoBuilder
	}

	name := filepath.Base(wd)
	return map[string]Interface{name: b}, nil

}

func (PNPMBasic) Build(ctx context.Context, client *dagger.Client, job *Job) (container *dagger.Container, err error) {
	defer errutil.Wrap("pnpm/basic", &err)

	cache := client.CacheVolume("pnpm-store-cache")
	host := client.Host().
		Directory(job.WorkDir, dagger.HostDirectoryOpts{Exclude: job.Excludes})

	builder := BaseImageForJob(client, job).
		WithExec([]string{"apk", "add", "--no-cache", "nodejs-current", "build-base", "python3"}).
		WithExec([]string{"corepack", "enable", "pnpm"}).
		WithMountedCache("/root/.local/share/pnpm", cache).
		WithFile("package.json", host.File("package.json")).
		WithFile("pnpm-lock.yaml", host.File("pnpm-lock.yaml")).
		WithExec([]string{"pnpm", "i"}).
		WithDirectory("/app", host).
		WithExec([]string{"pnpm", "build"})

	runner := BaseImageForJob(client, job).
		WithExec([]string{
			"apk", "add", "--no-cache",
			"nodejs-current", "tzdata", "ca-certificates",
		}).
		WithExec([]string{"corepack", "enable", "pnpm"}).
		WithFile("package.json", builder.File("package.json")).
		WithFile("pnpm-lock.yaml", builder.File("pnpm-lock.yaml")).
		WithExec([]string{"pnpm", "i"})

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

	runner = runner.WithDirectory("/app", builder.Directory(outdir)).
		WithDefaultArgs(dagger.ContainerWithDefaultArgsOpts{Args: args})

	if runner, err = runner.Sync(ctx); err != nil {
		return nil, err
	} else {
		runner.Export(ctx, job.Name+".docker")
		return runner, nil
	}
}
