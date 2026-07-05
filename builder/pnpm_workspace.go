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
)

type PNPMWorkspace struct{}

func (PNPMWorkspace) Name() string   { return "pnpm/workspace" }
func (PNPMWorkspace) Layout() Layout { return LayoutWorkspace }
func (PNPMWorkspace) Class() Class   { return ClassInterpreted }

func (b PNPMWorkspace) Discover(wd string) (map[string]Interface, error) {
	// PNPM decided to have a rename from pnpm-workspaces.yaml (with an s) to
	// just pnpm-workspace.yaml (without the s) and it'll actually throw an error for this
	// so we have to have this pointless detection to patch pnpm failure to backcompat
	if detected, err := fileutil.DetectFile(wd, "pnpm-workspace.yaml"); err != nil {
		return nil, err
	} else if !detected {
		if detected2, err := fileutil.DetectFile(wd, "pnpm-workspaces.yaml"); err != nil {
			return nil, err
		} else if !detected2 {
			return nil, ErrNoBuilder
		}
	}

	// scan for pnpm/basic on subfolders
	// TODO: Could just read the pnpm-workspace.yaml file and parse it as well, have not
	//   spend time to investigate if that is good enough or not yet so duplicating the
	//   logic from go/workspace for now
	mods := map[string]Interface{}
	err := fileutil.WalkSubdirs(wd, func(dir os.DirEntry) error {
		submods, err := PNPMBasic{}.Discover(filepath.Join(wd, dir.Name()))
		if errors.Is(err, ErrNoBuilder) {
			return nil
		}

		// found a pnpm/basic submodule, mark it as using pnpm/workspace
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

func (PNPMWorkspace) Build(ctx context.Context, client *dagger.Client, unit *BuildUnit) (container *dagger.Container, err error) {
	defer errutil.Wrap("pnpm/workspace", &err)

	wsdir, err := filepath.Abs(filepath.Join(unit.WorkDir, ".."))
	if err != nil {
		return nil, err
	}

	host := client.Host().Directory(wsdir, dagger.HostDirectoryOpts{
		Exclude: unit.Excludes,
	})

	// prepare job parameters
	outdir := strings.TrimSpace(unit.BuildDir)
	if outdir == "" {
		outdir = defaultBuildDir
	}

	cmd := strings.TrimSpace(unit.CommandName)
	if cmd == "" {
		cmd = defaultNodeBin
	}

	args := pnpmRunArgs(cmd, unit, ".")

	// build
	base := BaseImageForUnit(client, unit)
	base = withPNPMBase(base)
	base = withPNPMPkgCache(client, base)
	base = withUnitEnv(base, unit)
	base = base.WithWorkdir(SrcDir)

	builder := withBuildPkgs(base).
		WithDirectory(".", host).
		WithExec([]string{"pnpm", "-r", "install"}).
		WithExec([]string{"pnpm", "-r", "build"})

	// run
	runner := withRunnerPkgs(base).
		WithWorkdir(RunDir).
		WithDirectory(RunDir, builder.Directory(unit.Name+"/"+outdir)).
		WithDirectory(RunDir+"/node_modules", builder.Directory(unit.Name+"/node_modules"))

	runner = withPNPMModuleFix(runner)
	runner = withUnitAssets(runner, builder, unit)

	runner = runner.WithDefaultArgs(args)
	return runner.Sync(ctx)
}
