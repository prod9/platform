package framework

import (
	"context"
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/framework/fileutil"
	"platform.prodigy9.co/framework/gowork"
	"platform.prodigy9.co/framework/scaffold"
)

type GoWorkspace struct{ noScaffoldInputs }

func (GoWorkspace) Name() string   { return "go/workspace" }
func (GoWorkspace) Layout() Layout { return LayoutWorkspace }

func (GoWorkspace) Discover(wd string) bool {
	detected, _ := fileutil.DetectFile(wd, "go.work")
	return detected
}

func (fw GoWorkspace) Scaffold(ctx context.Context, wd, _, _ string, _ map[string]string) (scaffold.Spec, error) {
	return scaffold.Spec{Module: defaultModule(fw, wd)}, nil
}

func (GoWorkspace) Build(ctx context.Context, client *dagger.Client, unit *BuildUnit) (container *dagger.Container, err error) {
	defer errutil.Wrap("go/workspace", &err)

	wsdir, err := filepath.Abs(filepath.Join(unit.WorkDir, ".."))
	if err != nil {
		return nil, err
	}

	host := client.Host().Directory(wsdir, dagger.HostDirectoryOpts{
		Exclude: unit.Excludes,
	})

	workfile := filepath.Join(wsdir, "go.work")
	goversion, workmods, err := gowork.ParseFile(workfile)
	if err != nil {
		return nil, err
	}

	// prepare job parameters
	outbin := unit.Name

	cmd := strings.TrimSpace(unit.CommandName)
	if cmd == "" {
		cmd = outbin
	}
	args := append([]string{cmd}, unit.CommandArgs...)

	// build
	base := BaseImageForUnit(client, unit)

	builder := withBuildPkgs(base, "go").WithWorkdir(SrcDir)
	builder = withGoCaches(client, builder, goversion)
	builder = withGoVersion(builder, goversion)

	builder = builder.
		WithFile("go.work", host.File("go.work")).
		WithFile("go.work.sum", host.File("go.work.sum"))

	// needs all go.mod of all modules to start dependencies check
	// otherwise it'll try to fetch them from the internet during build
	for _, mod := range workmods {
		builder = builder.
			WithFile(SrcDir+"/"+mod+"/go.mod", host.File("./"+mod+"/go.mod")).
			WithFile(SrcDir+"/"+mod+"/go.sum", host.File("./"+mod+"/go.sum"))
	}

	// NOTE: Users should `go work sync` if mod doesn't match as build logs maybe invisible
	// or hard to track down for the user.
	builder = builder.
		WithExec([]string{"go", "mod", "download", "-x", "all"})

	testargs := []string{"go", "test", "-v"}
	for _, mod := range workmods {
		testargs = append(testargs, "./"+mod+"/...")
	}

	pkg := unit.PackageName
	if pkg == "" {
		pkg = "./" + unit.Name
	}

	builder = builder.
		WithDirectory(".", host).
		WithExec(testargs).
		WithExec([]string{"go", "build", "-v", "-o", BinDir + "/" + outbin, pkg})

	// run
	runner := withRunnerPkgs(base)
	runner = withUnitEnv(runner, unit)
	runner = runner.WithFile(BinDir+"/"+outbin, builder.File(BinDir+"/"+outbin))
	runner = withUnitAssets(runner, builder, unit)

	runner = runner.WithDefaultArgs(args)
	return runner.Sync(ctx)
}
