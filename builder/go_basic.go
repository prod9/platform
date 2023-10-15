package builder

import (
	"context"
	"runtime"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
)

var GoBasic = Builder{
	Name:  "go/basic",
	Build: buildGoBasic,
}

func buildGoBasic(ctx context.Context, client *dagger.Client, job *Job) (container *dagger.Container, err error) {
	defer errutil.Wrap("go/basic", &err)

	modcache := client.CacheVolume("go-" + runtime.Version() + "-modcache")
	host := client.Host().Directory(job.WorkDir, dagger.HostDirectoryOpts{
		Exclude: job.Excludes,
	})

	outname := "/" + job.BinaryName
	base := BaseImageForJob(client, job)

	builder := base.
		WithExec([]string{"apk", "add", "--no-cache", "build-base", "git", "go"}).
		WithMountedCache("/root/go/pkg/mod", modcache).
		WithFile("go.mod", host.File("go.mod")).
		WithFile("go.sum", host.File("go.sum")).
		WithExec([]string{"go", "mod", "download", "-x", "all"}).
		WithDirectory(".", host).
		WithExec([]string{"go", "test", "-v", "./..."}).
		WithExec([]string{"go", "build", "-v", "-o", outname, job.PackageName})

	runner := base.
		WithExec([]string{"apk", "add", "--no-cache", "ca-certificates", "tzdata"}).
		WithFile("/app/"+job.BinaryName, builder.File(outname)).
		WithDefaultArgs(dagger.ContainerWithDefaultArgsOpts{
			Args: append(
				[]string{"/app/" + job.BinaryName},
				job.BinaryArgs...,
			),
		})

	// TODO: Builder should probably report what binary are in the resulting container
	//   Because now we don't have a Dockerfile to look at
	return runner.Sync(ctx)
}
