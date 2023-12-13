package builder

import (
	"context"
	"path/filepath"
	"runtime"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/builder/fileutil"
)

type GoBasic struct{}

func (GoBasic) Name() string { return "go/basic" }
func (GoBasic) Kind() Kind   { return KindBasic }

func (b GoBasic) Discover(wd string) (map[string]Interface, error) {
	if detected, err := fileutil.DetectFile(wd, "go.mod"); err != nil {
		return nil, err
	} else if !detected {
		return nil, ErrNoBuilder
	}

	name := filepath.Base(wd)
	return map[string]Interface{name: b}, nil
}

func (GoBasic) Build(ctx context.Context, client *dagger.Client, job *Job) (container *dagger.Container, err error) {
	defer errutil.Wrap("go/basic", &err)

	modcache := client.CacheVolume("go-" + runtime.Version() + "-modcache")
	host := client.Host().Directory(job.WorkDir, dagger.HostDirectoryOpts{
		Exclude: job.Excludes,
	})

	outname := "/" + job.CommandName
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
		WithFile("/app/"+job.CommandName, builder.File(outname))

	for _, dir := range job.AssetDirs {
		runner = runner.WithDirectory(dir, builder.Directory(dir))
	}
	for key, value := range job.Env {
		runner = runner.WithEnvVariable(key, value)
	}

	runner = runner.WithDefaultArgs(dagger.ContainerWithDefaultArgsOpts{
		Args: append(
			[]string{"/app/" + job.CommandName},
			job.CommandArgs...,
		),
	})

	// TODO: Builder should probably report what binary are in the resulting container
	//   Because now we don't have a Dockerfile to look at
	return runner.Sync(ctx)
}
