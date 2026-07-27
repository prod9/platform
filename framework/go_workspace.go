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

type GoWorkspace struct{ noScaffoldVars }

var _ Framework = GoWorkspace{}

func (GoWorkspace) Name() string   { return "go/workspace" }
func (GoWorkspace) Layout() Layout { return LayoutWorkspace }

func (GoWorkspace) Discover(wd string) bool {
	return hasFile(wd, "go.work")
}

func (fw GoWorkspace) Scaffold(ctx context.Context, wd string, _ scaffold.Env, _ map[string]string) (scaffold.Spec, error) {
	return scaffold.Spec{Module: defaultModule(fw, wd)}, nil
}

func (GoWorkspace) Plan(*BuildUnit) []Step {
	return []Step{StepBase, StepDeps, StepTest, StepBuild, StepBuildRunner}
}

func (GoWorkspace) Execute(ctx context.Context, client *dagger.Client, unit *BuildUnit, step Step, in *dagger.Container) (container *dagger.Container, err error) {
	defer errutil.Wrap("go/workspace", &err)

	// The workspace root, one level up: the workspace file and sibling modules must come
	// into the build, then the target module is selected by name.
	wsdir, err := filepath.Abs(filepath.Join(unit.WorkDir, ".."))
	if err != nil {
		return nil, err
	}

	host := hostDir(client, wsdir, unit)
	outbin := unit.Name

	goversion, workmods, err := gowork.ParseFile(filepath.Join(wsdir, "go.work"))
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
		builder := in.
			WithFile("go.work", host.File("go.work")).
			WithFile("go.work.sum", host.File("go.work.sum"))

		// needs all go.mod of all modules to start dependencies check
		// otherwise it'll try to fetch them from the internet during build
		for _, mod := range workmods {
			builder = builder.
				WithFile(SrcDir+"/"+mod+"/go.mod", host.File("./"+mod+"/go.mod")).
				WithFile(SrcDir+"/"+mod+"/go.sum", host.File("./"+mod+"/go.sum"))
		}

		// NOTE: Users should `go work sync` if mod doesn't match as build logs maybe invisible
		// or hard to track down for the user.
		return builder.WithExec([]string{"go", "mod", "download", "-x", "all"}), nil

	case StepTest:
		testargs := []string{"go", "test", "-v"}
		for _, mod := range workmods {
			testargs = append(testargs, "./"+mod+"/...")
		}
		return in.WithDirectory(".", host).WithExec(testargs), nil

	case StepBuild:
		pkg := unit.PackageName
		if pkg == "" {
			pkg = "./" + unit.Name
		}
		return in.WithExec([]string{"go", "build", "-v", "-o", BinDir + "/" + outbin, pkg}), nil

	case StepBuildRunner:
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
