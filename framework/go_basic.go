package framework

import (
	"context"
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/framework/gowork"
	"platform.prodigy9.co/framework/scaffold"
)

type GoBasic struct{ noScaffoldInputs }

func (GoBasic) Name() string   { return "go/basic" }
func (GoBasic) Layout() Layout { return LayoutBasic }

func (GoBasic) Discover(wd string) bool {
	detected, _ := detectFile(wd, "go.mod")
	return detected
}

func (fw GoBasic) Scaffold(ctx context.Context, wd string, _ scaffold.Env, _ map[string]string) (scaffold.Spec, error) {
	return scaffold.Spec{Module: defaultModule(fw, wd)}, nil
}

func (GoBasic) Build(ctx context.Context, client *dagger.Client, unit *BuildUnit) (container *dagger.Container, err error) {
	defer errutil.Wrap("go/basic", &err)
	host := client.Host().Directory(unit.WorkDir, dagger.HostDirectoryOpts{
		Exclude: unit.Excludes,
	})

	// prepare job parameters
	outbin := unit.Name

	goversion, _, err := gowork.ParseFile(filepath.Join(unit.WorkDir, "go.mod"))
	if err != nil {
		return nil, err
	}

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
		WithFile("go.mod", host.File("go.mod")).
		WithFile("go.sum", host.File("go.sum")).
		WithExec([]string{"go", "mod", "download", "-x", "all"})

	builder = builder.
		WithDirectory(".", host).
		WithExec([]string{"go", "test", "-v", "./..."}).
		WithExec([]string{"go", "build", "-v", "-o", BinDir + "/" + outbin, unit.PackageName})

	// run
	runner := withRunnerPkgs(base)
	runner = withUnitEnv(runner, unit)
	runner = runner.WithFile(BinDir+"/"+outbin, builder.File(BinDir+"/"+outbin))
	runner = withUnitAssets(runner, builder, unit)

	runner = runner.WithDefaultArgs(args)
	return runner.Sync(ctx)
}
