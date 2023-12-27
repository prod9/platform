package builder

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/builder/fileutil"
	"platform.prodigy9.co/builder/gowork"
)

type GoWorkspace struct{}

func (GoWorkspace) Name() string { return "go/workspace" }
func (GoWorkspace) Kind() Kind   { return KindWorkspace }

func (b GoWorkspace) Discover(wd string) (map[string]Interface, error) {
	if detected, err := fileutil.DetectFile(wd, "go.work"); err != nil {
		return nil, err
	} else if !detected {
		return nil, ErrNoBuilder
	}

	// scan for go/basic on subfolders, should switch to proper go.work parsers if/when it
	// is available from go tooling directly
	mods := map[string]Interface{}
	err := fileutil.WalkSubdirs(wd, func(dir os.DirEntry) error {
		submods, err := GoBasic{}.Discover(filepath.Join(wd, dir.Name()))
		if errors.Is(err, ErrNoBuilder) {
			return nil
		}

		// found a go/basic submodule, mark it as using go/workspace
		for submod := range submods {
			mods[submod] = b
		}
		return nil
	})

	if err != nil {
		return nil, err
	} else if len(mods) == 0 {
		return nil, ErrNoBuilder
	} else {
		return mods, nil
	}
}

func (GoWorkspace) Build(ctx context.Context, client *dagger.Client, job *Job) (container *dagger.Container, err error) {
	defer errutil.Wrap("go/workspace", &err)

	// parse go.work file so we know what modules we need in the container
	rootdir := job.WorkDir
	wsdir, err := filepath.Abs(filepath.Join(rootdir, ".."))
	if err != nil {
		return nil, err
	}

	workfile := filepath.Join(wsdir, "go.work")
	goversion, workmods, err := gowork.ParseFile(workfile)
	if err != nil {
		return nil, err
	}

	if job.GoVersion != "" {
		goversion = job.GoVersion
	}

	gobin := "/root/sdk/go" + goversion + "/bin/go"
	modcache := client.CacheVolume("go-" + goversion + "-modcache")
	host := client.Host().Directory(wsdir, dagger.HostDirectoryOpts{
		Exclude: job.Excludes,
	})

	outname := job.CommandName
	base := BaseImageForJob(client, job)

	builder := base.
		WithExec([]string{"apk", "add", "--no-cache", "build-base", "go", "musl", "ca-certificates", "wget"}). //git
		WithExec([]string{"wget", "-q", "-O", "/etc/apk/keys/sgerrand.rsa.pub", "https://alpine-pkgs.sgerrand.com/sgerrand.rsa.pub"}).
		WithExec([]string{"wget", "https://github.com/sgerrand/alpine-pkg-glibc/releases/download/2.34-r0/glibc-2.34-r0.apk"}).
		WithExec([]string{"apk", "add", "--force-overwrite", "--no-cache", "glibc-2.34-r0.apk"}).
		WithMountedCache("/root/go/pkg/mod", modcache).
		WithEnvVariable("GOROOT", "/usr/lib/go").
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

	packagedir := filepath.Join("/app", job.PackageName)

	builder = builder.
		WithDirectory("/app", host).
		WithExec(testargs).
		WithExec([]string{gobin, "build", "-v", "-o", outname, packagedir})

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
		Args: append([]string{
			"/app/" + job.CommandName},
			job.CommandArgs...,
		),
	})

	return runner.Sync(ctx)

}
