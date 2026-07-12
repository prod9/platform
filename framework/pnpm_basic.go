package framework

import (
	"context"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/framework/fileutil"
	"platform.prodigy9.co/framework/scaffold"
)

type PNPMBasic struct{ noScaffoldInputs }

func (PNPMBasic) Name() string   { return "pnpm/basic" }
func (PNPMBasic) Layout() Layout { return LayoutBasic }

func (PNPMBasic) Discover(wd string) bool {
	detected, _ := fileutil.DetectFile(wd, "pnpm-lock.yaml")
	return detected
}

func (fw PNPMBasic) Scaffold(ctx context.Context, wd string) (scaffold.Spec, error) {
	return scaffold.Spec{Module: defaultModule(fw, wd)}, nil
}

func (PNPMBasic) Build(ctx context.Context, client *dagger.Client, unit *BuildUnit) (container *dagger.Container, err error) {
	defer errutil.Wrap("pnpm/basic", &err)

	host := client.Host().
		Directory(unit.WorkDir, dagger.HostDirectoryOpts{Exclude: unit.Excludes})

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
	base = base.
		WithWorkdir(SrcDir).
		WithFile("package.json", host.File("package.json")).
		WithFile("pnpm-lock.yaml", host.File("pnpm-lock.yaml")).
		WithExec([]string{"pnpm", "i"})

	builder := base.
		WithDirectory(".", host).
		WithExec([]string{"pnpm", "build"})

	// runner
	runner := withRunnerPkgs(base).
		WithWorkdir(RunDir).
		WithDirectory(RunDir, builder.Directory(outdir))
	runner = withUnitAssets(runner, builder, unit)

	runner = runner.WithDefaultArgs(args)
	return runner.Sync(ctx)
}
