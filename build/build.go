package build

import (
	"context"
	"os"
	"path/filepath"
	"runtime"

	"dagger.io/dagger"
	"github.com/go-git/go-git/v5"
)

func Build(job *Job) error {
	ctx, cancel := context.WithTimeout(context.Background(), job.Timeout)
	defer cancel()

	commit, err := GitCommit(job.WD)
	if err != nil {
		return err
	}

	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stderr))
	if err != nil {
		return err
	}
	defer client.Close()

	modcache := client.CacheVolume("go-" + runtime.Version() + "-modcache")
	host := client.Host().Directory(job.WD, dagger.HostDirectoryOpts{
		Exclude: []string{
			".git",
			"*.docker",
		},
	})

	base := client.Container(dagger.ContainerOpts{Platform: dagger.Platform(job.TargetPlatform)}).
		From("alpine:edge").
		WithWorkdir("/app").
		WithLabel("org.opencontainers.image.source", job.SourceURL)

	builder := base.
		WithExec([]string{"apk", "add", "--no-cache", "build-base", "go"}).
		WithMountedCache("/root/go/pkg/mod", modcache).
		WithFile("go.mod", host.File("go.mod")).
		WithFile("go.sum", host.File("go.sum")).
		WithExec([]string{"go", "mod", "download", "-x", "all"}).
		WithDirectory(".", host).
		WithExec([]string{"go", "test", "-v", "./..."}).
		WithExec([]string{"go", "build", "-v", "-o", job.BinaryName, job.PackageName})

	runner := base.
		WithExec([]string{"apk", "add", "--no-cache", "ca-certificates", "tzdata"}).
		WithFile(job.BinaryName, builder.File(job.BinaryName)).
		WithDefaultArgs(dagger.ContainerWithDefaultArgsOpts{
			Args: []string{"./" + job.BinaryName},
		})

	if _, err = runner.Publish(ctx, job.ImageName+":latest"); err != nil {
		return err
	} else if _, err = runner.Publish(ctx, job.ImageName+":"+commit); err != nil {
		return err
	}

	_, err = runner.Export(ctx, job.Name+".docker")
	return err
}

func GitCommit(wd string) (string, error) {
	if wd, err := filepath.Abs(wd); err != nil {
		return "", err
	} else if repo, err := git.PlainOpen(wd); err != nil {
		return "", err
	} else if ref, err := repo.Head(); err != nil {
		return "", err
	} else {
		return ref.Hash().String(), nil
	}
}
