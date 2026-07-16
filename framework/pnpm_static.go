package framework

import (
	"context"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/framework/scaffold"
)

type PNPMStatic struct{ noScaffoldInputs }

func (PNPMStatic) Name() string   { return "pnpm/static" }
func (PNPMStatic) Layout() Layout { return LayoutBasic }

func (PNPMStatic) Discover(wd string) bool {
	detected, _ := detectFile(wd, "astro.config.mjs")
	return detected
}

func (fw PNPMStatic) Scaffold(ctx context.Context, wd string, _ scaffold.Env, _ map[string]string) (scaffold.Spec, error) {
	return scaffold.Spec{Module: defaultModule(fw, wd)}, nil
}

func (PNPMStatic) Build(ctx context.Context, client *dagger.Client, unit *BuildUnit) (container *dagger.Container, err error) {
	defer errutil.Wrap("pnpm/static", &err)

	host := client.Host().
		Directory(unit.WorkDir, dagger.HostDirectoryOpts{Exclude: unit.Excludes})

	// prepare job parameters
	outdir := strings.TrimSpace(unit.BuildDir)
	if outdir == "" {
		outdir = defaultBuildDir
	}

	cmd := strings.TrimSpace(unit.CommandName)
	if cmd == "" {
		cmd = "caddy"
	}

	args := pnpmRunArgs(cmd, unit, "file-server", "-l", "0.0.0.0:3000")

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
	runner := withRunnerPkgs(base)
	runner = withCaddyServer(runner).
		WithWorkdir(RunDir).
		WithDirectory(RunDir, builder.Directory(outdir))
	runner = withUnitAssets(runner, builder, unit)

	runner = runner.WithDefaultArgs(args)
	return runner.Sync(ctx)
}
