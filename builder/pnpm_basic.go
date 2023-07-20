package builder

import (
	"context"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
)

var PNPMBasic = Builder{
	Name:  "pnpm/basic",
	Build: buildPNPMBasic,
}

func buildPNPMBasic(ctx context.Context, client *dagger.Client, job *Job) (container *dagger.Container, err error) {
	defer errutil.Wrap("pnpm/basic", &err)

	cache := client.CacheVolume("pnpm-store-cache")
	host := client.Host().
		Directory(job.WorkDir, dagger.HostDirectoryOpts{Exclude: job.Excludes})

	builder := BaseImageForJob(client, job).
		WithExec([]string{"apk", "add", "--no-cache", "nodejs-current"}).
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
