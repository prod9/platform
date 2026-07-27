package framework

import (
	"context"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/framework/scaffold"
)

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
		return pnpmBase(client, unit), nil

	case StepDeps:
		return withPNPMDeps(in, host), nil

	case StepBuild:
		return in.WithDirectory(".", host).WithExec([]string{"pnpm", "build"}), nil

	case StepBuildRunner:
		outdir := strings.TrimSpace(unit.BuildDir)
		if outdir == "" {
			outdir = defaultBuildDir
		}

		// Static family: only the built bundle and a webserver ship, no language runtime.
		runner := withRunnerPkgs(withPNPMDeps(pnpmBase(client, unit), host))
		runner = withCaddyServer(runner, unit).
			WithWorkdir(RunDir).
			WithDirectory(RunDir, in.Directory(outdir))
		runner = withUnitAssets(runner, in, unit)

		cmd := strings.TrimSpace(unit.CommandName)
		if cmd == "" {
			cmd = "caddy"
		}
		return runner.WithDefaultArgs(pnpmRunArgs(cmd, unit, caddyRunArgs()...)), nil

	default:
		return nil, unknownStep(step)
	}
}
