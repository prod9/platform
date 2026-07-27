package framework

import (
	"context"
	"strings"

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
		outdir := strings.TrimSpace(unit.BuildDir)
		if outdir == "" {
			outdir = defaultBuildDir
		}

		// StepBase's provisioning, repeated in the same order so Dagger dedupes the identical
		// prefix instead of installing pnpm a second time. Re-derived rather than threaded.
		runner := BaseImageForUnit(client, unit)
		runner = withBuildPkgs(runner)
		runner = withPNPM(runner)
		runner = withPNPMPkgCache(client, runner)
		runner = withUnitEnv(runner, unit)

		// Runner packages ahead of the dependency layer: they never change, while the
		// dependency layer keys on the repo's manifests. This is an interpreted family, so
		// node_modules and the runtime stay in the image rather than being left behind.
		runner = withRunnerPkgs(runner)
		runner = withPNPMDeps(runner, host).
			WithWorkdir(RunDir).
			WithDirectory(RunDir, in.Directory(outdir))
		runner = withUnitAssets(runner, in, unit)

		cmd := strings.TrimSpace(unit.CommandName)
		if cmd == "" {
			cmd = defaultNodeBin
		}
		return runner.WithDefaultArgs(pnpmRunArgs(cmd, unit, ".")), nil

	default:
		return nil, unknownStep(step)
	}
}
