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

type GoBasic struct{ noScaffoldVars }

var _ Framework = GoBasic{}

func (GoBasic) Name() string   { return "go/basic" }
func (GoBasic) Layout() Layout { return LayoutBasic }

func (GoBasic) Discover(wd string) bool {
	return hasFile(wd, "go.mod")
}

func (fw GoBasic) Scaffold(ctx context.Context, wd string, _ scaffold.Env, _ map[string]string) (scaffold.Spec, error) {
	return scaffold.Spec{Module: defaultModule(fw, wd)}, nil
}

func (GoBasic) Plan(*BuildUnit) []Step {
	return []Step{StepBase, StepDeps, StepTest, StepBuild, StepBuildRunner}
}

func (GoBasic) Execute(ctx context.Context, client *dagger.Client, unit *BuildUnit, step Step, in *dagger.Container) (container *dagger.Container, err error) {
	defer errutil.Wrap("go/basic", &err)

	host := unitHost(client, unit)
	outbin := unit.Name

	goversion, _, err := gowork.ParseFile(filepath.Join(unit.WorkDir, "go.mod"))
	if err != nil {
		return nil, err
	}

	switch step {
	case StepBase:
		builder := BaseImageForUnit(client, unit)
		builder = withBuildPkgs(builder, "go").
			WithWorkdir(SrcDir)
		builder = withGoCaches(client, builder, goversion)
		builder = withGoVersion(builder, goversion)
		return builder, nil

	case StepDeps:
		// go.mod/go.sum alone, so the dependency layer keys on the manifests and survives
		// every source edit.
		return in.
			WithFile("go.mod", host.File("go.mod")).
			WithFile("go.sum", host.File("go.sum")).
			WithExec([]string{"go", "mod", "download", "-x", "all"}), nil

	case StepTest:
		// Source lands here: the tests are the first stage that needs it, and Dagger fails
		// the build on a non-zero exec, which is what makes green tests a precondition.
		return in.
			WithDirectory(".", host).
			WithExec([]string{"go", "test", "-v", "./..."}), nil

	case StepBuild:
		return in.WithExec([]string{"go", "build", "-v", "-o", BinDir + "/" + outbin, unit.PackageName}), nil

	case StepBuildRunner:
		// The base is a pure function of (client, unit) and Dagger dedupes it, so the
		// runner re-derives its own rather than needing the builder's ancestor handed back.
		runner := BaseImageForUnit(client, unit)
		runner = withRunnerPkgs(runner)
		runner = withUnitEnv(runner, unit)
		runner = runner.WithFile(BinDir+"/"+outbin, in.File(BinDir+"/"+outbin))
		runner = withUnitAssets(runner, in, unit)

		cmd := strings.TrimSpace(unit.CommandName)
		if cmd == "" {
			cmd = outbin
		}
		return runner.WithDefaultArgs(append([]string{cmd}, unit.CommandArgs...)), nil

	default:
		return nil, unknownStep(step)
	}
}
