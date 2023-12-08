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

	host := client.Host().
		Directory(job.WorkDir, dagger.HostDirectoryOpts{Exclude: job.Excludes})

	base := BaseImageForJob(client, job).
		WithExec([]string{"apk", "add", "--no-cache", "nodejs-current", "build-base", "python3"}).
		WithExec([]string{"corepack", "enable", "pnpm"}).
		WithDirectory("/app", host)

	builder := base.
		WithExec([]string{"pnpm", "i"}).
		WithExec([]string{"pnpm", "--filter=" + job.PackageName, "build"})

	runner := builder.
		WithExec([]string{
			"apk", "add", "--no-cache",
			"nodejs-current", "tzdata", "ca-certificates",
		}).
		WithDefaultArgs(dagger.ContainerWithDefaultArgsOpts{
			Args: []string{"/usr/bin/node", filepath.Join(job.PackageName, "build")},
		})

	return runner.Sync(ctx)
}
