package framework

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"gopkg.in/yaml.v3"
	"platform.prodigy9.co/framework/scaffold"
)

type PNPMWorkspace struct{ noScaffoldVars }

var _ Framework = PNPMWorkspace{}

func (PNPMWorkspace) Name() string   { return "pnpm/workspace" }
func (PNPMWorkspace) Layout() Layout { return LayoutWorkspace }

// Discover keys on the packages list, never on the file. From pnpm v10 every repo
// carries pnpm-workspace.yaml — it holds all non-auth settings, including the build
// approvals a single-package repo must commit — so presence proves nothing. packages
// is what pnpm itself calls a workspace.
func (PNPMWorkspace) Discover(wd string) bool {
	body, err := os.ReadFile(filepath.Join(wd, "pnpm-workspace.yaml"))
	if err != nil {
		return false
	}

	settings := struct {
		Packages []string `yaml:"packages"`
	}{}
	if err := yaml.Unmarshal(body, &settings); err != nil {
		return false
	}

	return len(settings.Packages) > 0
}

func (fw PNPMWorkspace) Scaffold(ctx context.Context, wd string, _ scaffold.Env, _ map[string]string) (scaffold.Spec, error) {
	return scaffold.Spec{Module: defaultModule(fw, wd)}, nil
}

func (PNPMWorkspace) Plan(*BuildUnit) []Step {
	return []Step{StepBase, StepDeps, StepBuild, StepBuildRunner}
}

func (PNPMWorkspace) Execute(ctx context.Context, client *dagger.Client, unit *BuildUnit, step Step, in *dagger.Container) (container *dagger.Container, err error) {
	defer errutil.Wrap("pnpm/workspace", &err)

	// The workspace root, one level up: pnpm-workspace.yaml and the sibling packages must
	// come into the build; the target module is selected by name below.
	wsdir, err := filepath.Abs(filepath.Join(unit.WorkDir, ".."))
	if err != nil {
		return nil, err
	}

	host := hostDir(client, wsdir, unit)

	switch step {
	case StepBase:
		builder := BaseImageForUnit(client, unit)
		builder = withBuildPkgs(builder)
		builder = withPNPM(builder)
		builder = withPNPMPkgCache(client, builder)

		builder = withUnitEnv(builder, unit).
			WithWorkdir(SrcDir)
		return builder, nil

	case StepDeps:
		// A workspace installs from the whole tree, not from filtered manifests: pnpm
		// resolves the members' interdependencies, so the sources must already be there.
		return withBuildPkgs(in).
			WithDirectory(".", host).
			WithExec([]string{"pnpm", "-r", "install"}), nil

	case StepBuild:
		return in.WithExec([]string{"pnpm", "-r", "build"}), nil

	case StepBuildRunner:
		outdir := strings.TrimSpace(unit.BuildDir)
		if outdir == "" {
			outdir = defaultBuildDir
		}

		// StepBase's provisioning, repeated in the same order so Dagger dedupes the identical
		// prefix instead of installing pnpm a second time.
		runner := BaseImageForUnit(client, unit)
		runner = withBuildPkgs(runner)
		runner = withPNPM(runner)
		runner = withPNPMPkgCache(client, runner)
		runner = withUnitEnv(runner, unit).
			WithWorkdir(SrcDir)

		runner = withRunnerPkgs(runner).
			WithWorkdir(RunDir).
			WithDirectory(RunDir, in.Directory(unit.Name+"/"+outdir)).
			WithDirectory(RunDir+"/node_modules", in.Directory(unit.Name+"/node_modules"))
		runner = withPNPMModuleFix(runner)
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
