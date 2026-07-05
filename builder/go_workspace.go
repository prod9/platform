package builder

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/builder/fileutil"
	"platform.prodigy9.co/builder/gowork"
)

type GoWorkspace struct{}

func (GoWorkspace) Name() string   { return "go/workspace" }
func (GoWorkspace) Layout() Layout { return LayoutWorkspace }
func (GoWorkspace) Class() Class   { return ClassNative }

func (b GoWorkspace) Discover(wd string) (map[string]Interface, error) {
	if detected, err := fileutil.DetectFile(wd, "go.work"); err != nil {
		return nil, err
	} else if !detected {
		return nil, ErrNoBuilder
	}

	// scan for go/basic on subfolders, should switch to proper go.work parsers if/when it
	// is available from go tooling directly
	mods := map[string]Interface{}
	err := fileutil.WalkSubdirs(wd, func(dir os.DirEntry) error {
		submods, err := GoBasic{}.Discover(filepath.Join(wd, dir.Name()))
		if errors.Is(err, ErrNoBuilder) {
			return nil
		}

		// found a go/basic submodule, mark it as using go/workspace
		for submod := range submods {
			mods[submod] = b
		}
		return nil
	})

	if err != nil {
		return nil, err
	} else if len(mods) == 0 {
		return nil, ErrNoBuilder
	} else {
		return mods, nil
	}
}

func (GoWorkspace) Build(ctx context.Context, client *dagger.Client, unit *BuildUnit) (container *dagger.Container, err error) {
	defer errutil.Wrap("go/workspace", &err)

	wsdir, err := filepath.Abs(filepath.Join(unit.WorkDir, ".."))
	if err != nil {
		return nil, err
	}

	host := client.Host().Directory(wsdir, dagger.HostDirectoryOpts{
		Exclude: unit.Excludes,
	})

	workfile := filepath.Join(wsdir, "go.work")
	goversion, workmods, err := gowork.ParseFile(workfile)
	if err != nil {
		return nil, err
	}

	// prepare job parameters
	outbin := unit.Name

	cmd := strings.TrimSpace(unit.CommandName)
	if cmd == "" {
		cmd = outbin
	}
	args := append([]string{cmd}, unit.CommandArgs...)

	// build
	base := BaseImageForUnit(client, unit)

	builder := withBuildPkgs(base, "go").WithWorkdir(SrcDir)
	builder, gobin := withGoVersion(builder, goversion)
	builder = withGoPkgCache(client, builder, goversion)

	builder = builder.
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
	builder = builder.
		WithExec([]string{gobin, "mod", "download", "-x", "all"})

	testargs := []string{gobin, "test", "-v"}
	for _, mod := range workmods {
		testargs = append(testargs, "./"+mod+"/...")
	}

	pkg := unit.PackageName
	if pkg == "" {
		pkg = "./" + unit.Name
	}

	builder = builder.
		WithDirectory(".", host).
		WithExec(testargs).
		WithExec([]string{gobin, "build", "-v", "-o", BinDir + "/" + outbin, pkg})

	// run
	runner := withRunnerPkgs(base)
	runner = withUnitEnv(runner, unit)
	runner = runner.WithFile(BinDir+"/"+outbin, builder.File(BinDir+"/"+outbin))
	for _, dir := range unit.AssetDirs {
		runner = runner.WithDirectory(dir, builder.Directory(dir))
	}

	runner = runner.WithDefaultArgs(args)
	return runner.Sync(ctx)
}
