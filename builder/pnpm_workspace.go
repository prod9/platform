package builder

import (
	"context"
	"path/filepath"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
)

var PNPMWorkspace = Builder{
	Name:  "pnpm/workspace",
	Build: buildPNPMWorkspace,
}

func buildPNPMWorkspace(ctx context.Context, client *dagger.Client, job *Job) (container *dagger.Container, err error) {
	defer errutil.Wrap("pnpm/workspace", &err)

	cache := client.CacheVolume("pnpm-store-cache")
	host := client.Host().
		Directory(job.WorkDir, dagger.HostDirectoryOpts{Exclude: job.Excludes})

	base := BaseImageForJob(client, job).
		WithExec([]string{"apk", "add", "--no-cache", "nodejs-current", "build-base", "python3"}).
		WithExec([]string{"corepack", "enable", "pnpm"}).
		WithDirectory("/app", host)

	deploydir := filepath.Join("/prod", job.PackageName)

	builder := base.
		WithMountedCache("/pnpm/store", cache).
		WithExec([]string{"pnpm", "i"}).
		WithWorkdir(filepath.Join("/app", job.PackageName)).
		WithExec([]string{"pnpm", "build"}).
		WithWorkdir("/app").
		WithExec([]string{"pnpm", "deploy", "--filter=" + job.PackageName, "--prod", deploydir}).
		WithExec([]string{"pnpm", "prune", "--prod"})

	runner := base.
		WithExec([]string{
			"apk", "add", "--no-cache",
			"nodejs-current", "tzdata", "ca-certificates",
		}).
		WithDirectory("/app", builder.Directory(deploydir)).
		WithDefaultArgs(dagger.ContainerWithDefaultArgsOpts{
			Args: []string{"/usr/bin/node", filepath.Join("/app", job.PackageName, "build")},
		})

	return runner.Sync(ctx)
}
