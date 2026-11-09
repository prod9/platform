package framework

import (
	"context"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/framework/scaffold"
)

type PNPMBasic struct{ noScaffoldVars }

var _ Framework = PNPMBasic{}

func (PNPMBasic) Name() string   { return "pnpm/basic" }
func (PNPMBasic) Layout() Layout { return LayoutBasic }

func (PNPMBasic) Discover(wd string) bool {
	return hasFile(wd, "pnpm-lock.yaml")
}

func (fw PNPMBasic) Scaffold(ctx context.Context, wd string, _ scaffold.Env, _ map[string]string) (scaffold.Spec, error) {
	return scaffold.Spec{Module: defaultModule(fw, wd)}, nil
}

func (PNPMBasic) Plan(*BuildUnit) []Step {
	return []Step{StepBase, StepDeps, StepBuild, StepBuildRunner}
}

func (PNPMBasic) Execute(ctx context.Context, client *dagger.Client, unit *BuildUnit, step Step, in *dagger.Container) (container *dagger.Container, err error) {
	defer errutil.Wrap("pnpm/basic", &err)

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
		// SvelteKit adapter-node's default `out`, the app shape pnpm/basic serves.
		const defaultBuildDir = "build"
		outdir := buildDir(unit, defaultBuildDir)

		// StepBase and StepDeps, repeated call for call so Dagger reuses their layers: the
		// runner's node_modules is the build's own install, never a second one and never a
		// copy from another image. Re-derived rather than threaded.
		runner := BaseImageForUnit(client, unit)
		runner = withBuildPkgs(runner)
		runner = withPNPM(runner)
		runner = withPNPMPkgCache(client, runner)
		runner = withUnitEnv(runner, unit)
		runner = withPNPMDeps(runner, host)

		// Node resolves the bundle's bare imports by walking up from RunDir, so the
		// install in SrcDir is linked in beside it (docs/spec/frameworks.md).
		runner = withRunnerPkgs(runner).
			WithWorkdir(RunDir).
			WithDirectory(RunDir, in.Directory(outdir)).
			WithSymlink(SrcDir+"/node_modules", RunDir+"/node_modules")
		runner = withUnitAssets(runner, in, unit)

		return runner.WithDefaultArgs(runArgs(unit, defaultNodeBin, ".")), nil

	default:
		return nil, unknownStep(step)
	}
}
