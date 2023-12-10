package builder

import (
	"context"
	"path/filepath"

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

	runner := builder.
		WithExec([]string{
			"apk", "add", "--no-cache",
			"nodejs-current", "tzdata", "ca-certificates",
		}).
		WithDirectory("/app", builder.Directory("build")).
		WithFile("package.json", builder.File("package.json")).
		WithFile("pnpm-lock.yaml", builder.File("pnpm-lock.yaml")).
		WithDefaultArgs(dagger.ContainerWithDefaultArgsOpts{
			Args: []string{"/usr/bin/node", "."},
		})

	return runner.Sync(ctx)
}
