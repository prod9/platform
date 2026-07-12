package framework

import (
	"context"
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/framework/fileutil"
	"platform.prodigy9.co/framework/scaffold"
)

type PNPMWorkspace struct{ noScaffoldInputs }

func (PNPMWorkspace) Name() string   { return "pnpm/workspace" }
func (PNPMWorkspace) Layout() Layout { return LayoutWorkspace }

func (PNPMWorkspace) Discover(wd string) bool {
	detected, _ := fileutil.DetectFile(wd, "pnpm-workspace.yaml")
	if !detected {
		detected, _ = fileutil.DetectFile(wd, "pnpm-workspaces.yaml")
	}
	return detected
}

func (fw PNPMWorkspace) Scaffold(ctx context.Context, wd string) (scaffold.Spec, error) {
	return scaffold.Spec{Module: defaultModule(fw, wd)}, nil
}

func (PNPMWorkspace) Build(ctx context.Context, client *dagger.Client, unit *BuildUnit) (container *dagger.Container, err error) {
	defer errutil.Wrap("pnpm/workspace", &err)

	wsdir, err := filepath.Abs(filepath.Join(unit.WorkDir, ".."))
	if err != nil {
		return nil, err
	}

	host := client.Host().Directory(wsdir, dagger.HostDirectoryOpts{
		Exclude: unit.Excludes,
	})

	// prepare job parameters
	outdir := strings.TrimSpace(unit.BuildDir)
	if outdir == "" {
		outdir = defaultBuildDir
	}

	cmd := strings.TrimSpace(unit.CommandName)
	if cmd == "" {
		cmd = defaultNodeBin
	}

	args := pnpmRunArgs(cmd, unit, ".")

	// build
	base := BaseImageForUnit(client, unit)
	base = withPNPMBase(base)
	base = withPNPMPkgCache(client, base)
	base = withUnitEnv(base, unit)
	base = base.WithWorkdir(SrcDir)

	builder := withBuildPkgs(base).
		WithDirectory(".", host).
		WithExec([]string{"pnpm", "-r", "install"}).
		WithExec([]string{"pnpm", "-r", "build"})

	// run
	runner := withRunnerPkgs(base).
		WithWorkdir(RunDir).
		WithDirectory(RunDir, builder.Directory(unit.Name+"/"+outdir)).
		WithDirectory(RunDir+"/node_modules", builder.Directory(unit.Name+"/node_modules"))

	runner = withPNPMModuleFix(runner)
	runner = withUnitAssets(runner, builder, unit)

	runner = runner.WithDefaultArgs(args)
	return runner.Sync(ctx)
}
