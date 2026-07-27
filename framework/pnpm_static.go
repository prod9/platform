package framework

import (
	"context"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/framework/scaffold"
)

// astroBuildDir is Astro's own default outDir, and Discover keys on astro.config.mjs — so
// a project that names no build_dir is an Astro project taking Astro's default.
const astroBuildDir = "dist"

type PNPMStatic struct{ noScaffoldVars }

var _ Framework = PNPMStatic{}

func (PNPMStatic) Name() string   { return "pnpm/static" }
func (PNPMStatic) Layout() Layout { return LayoutBasic }

func (PNPMStatic) Discover(wd string) bool {
	return hasFile(wd, "astro.config.mjs")
}

func (fw PNPMStatic) Scaffold(ctx context.Context, wd string, _ scaffold.Env, _ map[string]string) (scaffold.Spec, error) {
	return scaffold.Spec{Module: defaultModule(fw, wd)}, nil
}

func (PNPMStatic) Plan(*BuildUnit) []Step {
	return []Step{StepBase, StepDeps, StepBuild, StepBuildRunner}
}

func (PNPMStatic) Execute(ctx context.Context, client *dagger.Client, unit *BuildUnit, step Step, in *dagger.Container) (container *dagger.Container, err error) {
	defer errutil.Wrap("pnpm/static", &err)

	host := unitHost(client, unit)

	switch step {
	case StepBase:
		builder := BaseImageForUnit(client, unit)
		builder = withBuildPkgs(builder)
		builder = withPNPM(builder)
		builder = withPNPMPkgCache(client, builder)

		builder = withUnitEnv(builder, unit)
		return builder, nil

	case StepDeps:
		return withPNPMDeps(in, host), nil

	case StepBuild:
		return in.WithDirectory(".", host).WithExec([]string{"pnpm", "build"}), nil

	case StepBuildRunner:
		outdir := buildDir(unit, astroBuildDir)

		// Static family: only the built bundle and a webserver ship. Off the bare base, never
		// the pnpm one — the runtime, corepack, the build packages and node_modules all
		// belong to the build, and nothing serves files with them.
		runner := BaseImageForUnit(client, unit)
		runner = withRunnerPkgs(runner)
		runner = withCaddyServer(runner)
		runner = withUnitEnv(runner, unit).
			WithWorkdir(RunDir).
			WithDirectory(RunDir, in.Directory(outdir))
		runner = withUnitAssets(runner, in, unit)

		return runner.WithDefaultArgs(runArgs(unit, "caddy", "run", "--config", caddyfilePath)), nil

	default:
		return nil, unknownStep(step)
	}
}
