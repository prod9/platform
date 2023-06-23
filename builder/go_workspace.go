package builder

import (
	"context"
	"path/filepath"
	"runtime"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/builder/gowork"
)

var GoWorkspace = Builder{
	Name:  "go/workspace",
	Build: buildGoWorkspace,
}

func buildGoWorkspace(ctx context.Context, client *dagger.Client, job *Job) (err error) {
	defer errutil.Wrap("go/workspace", &err)

	// parse go.work file so we know what modules we need in the container
	rootdir := filepath.Dir(job.Config.ConfigPath)
	workfile := filepath.Join(rootdir, "go.work")
	workmods, err := gowork.ParseFile(workfile)
	if err != nil {
		return err
	}

	modcache := client.CacheVolume("go-" + runtime.Version() + "-modcache")
	host := client.Host().Directory(rootdir, dagger.HostDirectoryOpts{
		Exclude: job.Excludes,
	})

	outname := "/" + job.BinaryName
	base := BaseImageForJob(client, job)

	builder := base.
		WithExec([]string{"apk", "add", "--no-cache", "build-base", "go"}).
		WithMountedCache("/root/go/pkg/mod", modcache).
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
		WithExec([]string{"go", "mod", "download", "-x", "all"})

	// test and build
	testargs := []string{"go", "test", "-v"}
	for _, mod := range workmods {
		testargs = append(testargs, "./"+mod+"/...")
	}

	builder = builder.
		WithDirectory("/app", host).
		WithExec(testargs).
		WithExec([]string{"go", "build", "-v", "-o", outname, job.PackageName})

	runner := base.
		WithExec([]string{"apk", "add", "--no-cache", "ca-certificates", "tzdata"}).
		WithFile("/app/"+job.BinaryName, builder.File(outname)).
		WithDefaultArgs(dagger.ContainerWithDefaultArgsOpts{
			Args: []string{"/app/" + job.BinaryName},
		})

	runner, err = runner.Sync(ctx)
	// TODO: Publish runner
	return err

}
