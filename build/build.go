package build

import (
	"context"
	"os"
	"runtime"
	"time"

	"dagger.io/dagger"
)

const (
	Timeout        = 5 * time.Minute
	TargetPlatform = "linux/amd64"
	ImageName      = "ghcr.io/prod9/platform"
	PackageName    = "platform.prodigy9.co"
	BinaryName     = "platform"
)

func Build(target string) error {
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()

	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stderr))
	if err != nil {
		return err
	}
	defer client.Close()

	modcache := client.CacheVolume("go-" + runtime.Version() + "-modcache")

	host := client.Host().Directory(".", dagger.HostDirectoryOpts{
		Exclude: []string{
			".git",
			"*.docker",
		},
	})

	builder := client.Container(dagger.ContainerOpts{Platform: TargetPlatform}).
		From("alpine:edge").
		WithWorkdir("/app").
		WithExec([]string{"apk", "add", "--no-cache", "build-base", "go"}).
		WithMountedCache("/root/go/pkg/mod", modcache).
		WithFile("go.mod", host.File("go.mod")).
		WithFile("go.sum", host.File("go.sum")).
		WithExec([]string{"go", "mod", "download", "-x", "all"}).
		WithDirectory(".", host).
		WithExec([]string{"go", "build", "-v", "-o", BinaryName, PackageName})

	runner := client.Container(dagger.ContainerOpts{Platform: TargetPlatform}).
		From("alpine:edge").
		WithWorkdir("/app").
		WithExec([]string{"apk", "add", "--no-cache", "ca-certificates", "tzdata"}).
		WithFile(BinaryName, builder.File(BinaryName)).
		WithDefaultArgs(dagger.ContainerWithDefaultArgsOpts{
			Args: []string{"./" + BinaryName},
		})

	runner.Publish(ctx, ImageName+":latest")

	_, err = runner.Export(ctx, "platform.docker")
	return err
}
