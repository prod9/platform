package builder

import (
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/builder/fileutil"
	"platform.prodigy9.co/builder/gowork"
)

type GoBasic struct{}

func (GoBasic) Name() string   { return "go/basic" }
func (GoBasic) Layout() Layout { return LayoutBasic }
func (GoBasic) Class() Class   { return ClassNative }

func (b GoBasic) Discover(wd string) (map[string]Interface, error) {
	if detected, err := fileutil.DetectFile(wd, "go.mod"); err != nil {
		return nil, err
	} else if !detected {
		return nil, ErrNoBuilder
	}

	name := filepath.Base(wd)
	return map[string]Interface{name: b}, nil
}

func (GoBasic) Build(eng Engine, unit *BuildUnit) (container *dagger.Container, err error) {
	defer errutil.Wrap("go/basic", &err)
	host := eng.Client().Host().Directory(unit.WorkDir, dagger.HostDirectoryOpts{
		Exclude: unit.Excludes,
	})

	// prepare job parameters
	cmd := strings.TrimSpace(unit.CommandName)
	switch {
	case cmd == "" && unit.PackageName != "":
		cmd = unit.PackageName
	case cmd == "" && unit.Name != "":
		cmd = unit.Name
	}

	goversion, _, err := gowork.ParseFile(filepath.Join(unit.WorkDir, "go.mod"))
	if err != nil {
		return nil, err
	}

	args := []string{"./" + cmd}
	if len(unit.CommandArgs) > 0 {
		args = append(args, unit.CommandArgs...)
	}

	// build
	base := BaseImageForUnit(eng, unit)
	builder := withBuildPkgs(base, "go")
	builder, gobin := withGoVersion(builder, goversion)
	builder = withGoPkgCache(eng, builder, goversion)

	builder = builder.
		WithFile("go.mod", host.File("go.mod")).
		WithFile("go.sum", host.File("go.sum")).
		WithExec([]string{gobin, "mod", "download", "-x", "all"})

	builder = builder.
		WithDirectory(".", host).
		WithExec([]string{gobin, "test", "-v", "./..."}).
		WithExec([]string{gobin, "build", "-v", "-o", "/out/" + cmd, unit.PackageName})

	// run
	runner := withRunnerPkgs(base)
	runner = withUnitEnv(runner, unit)
	runner = runner.WithFile("/app/"+cmd, builder.File("/out/"+cmd))
	for _, dir := range unit.AssetDirs {
		runner = runner.WithDirectory(dir, builder.Directory(dir))
	}

	runner = runner.WithDefaultArgs(args)
	return runner.Sync(eng.Context())
}
