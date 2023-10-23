package builder

import (
	"context"
	"path/filepath"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/builder/gowork"
)

var GoWorkspace = Builder{
	Name:  "go/workspace",
	Build: buildGoWorkspace,
}

func buildGoWorkspace(ctx context.Context, client *dagger.Client, job *Job) (container *dagger.Container, err error) {
	defer errutil.Wrap("go/workspace", &err)

	// parse go.work file so we know what modules we need in the container
	rootdir := job.Config.ConfigDir
	workfile := filepath.Join(job.Config.ConfigDir, "go.work")
	goversion, workmods, err := gowork.ParseFile(workfile)
	if err != nil {
		return nil, err
	}

	gobin := "/root/sdk/go" + goversion + "/bin/go"
	modcache := client.CacheVolume("go-" + goversion + "-modcache")
	host := client.Host().Directory(rootdir, dagger.HostDirectoryOpts{
		Exclude: job.Excludes,
	})

	outname := "/" + job.BinaryName
	base := BaseImageForJob(client, job)

	builder := base.
		WithExec([]string{"apk", "add", "--no-cache", "build-base", "go"}). //git
		WithMountedCache("/root/go/pkg/mod", modcache).
		WithExec([]string{"go", "install", "golang.org/dl/go" + goversion + "@latest"}).
		WithExec([]string{"/root/go/bin/go" + goversion, "download"}).
		WithFile("go.work", host.File("go.work")).
		WithFile("go.work.sum", host.File("go.work.sum"))

	// needs all go.mod of all modules to start dependencies check
	// otherwise it'll try to fetch them from the internet during build
	for _, mod := range workmods {
		builder = builder.
			WithFile("/app/"+mod+"/go.mod", host.File("./"+mod+"/go.mod")).
			WithFile("/app/"+mod+"/go.sum", host.File("./"+mod+"/go.sum"))
	}

	// NOTE: Users should `go work sync` if mod doesn't match as build logs maybe invisible
	// or hard to track down for the user.
	builder = builder.
		WithExec([]string{gobin, "mod", "download", "-x", "all"})

	// test and build
	testargs := []string{gobin, "test", "-v"}
	for _, mod := range workmods {
		testargs = append(testargs, "./"+mod+"/...")
	}

	builder = builder.
		WithDirectory("/app", host).
		WithExec(testargs).
		WithExec([]string{gobin, "build", "-v", "-o", outname, job.PackageName})

	runner := base.
		WithExec([]string{"apk", "add", "--no-cache", "ca-certificates", "tzdata"}).
		WithFile("/app/"+job.BinaryName, builder.File(outname))

	for _, dir := range job.AssetDirs {
		runner = runner.WithDirectory(dir, builder.Directory(dir))
	}
	for key, value := range job.Env {
		runner = runner.WithEnvVariable(key, value)
	}

	runner = runner.WithDefaultArgs(dagger.ContainerWithDefaultArgsOpts{
		Args: []string{"/app/" + job.BinaryName},
	})

	return runner.Sync(ctx)

}
