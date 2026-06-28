package builder

import (
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/builder/fileutil"
)

type PNPMBasic struct{}

func (PNPMBasic) Name() string   { return "pnpm/basic" }
func (PNPMBasic) Layout() Layout { return LayoutBasic }
func (PNPMBasic) Class() Class   { return ClassInterpreted }

func (b PNPMBasic) Discover(wd string) (map[string]Interface, error) {
	if detected, err := fileutil.DetectFile(wd, "pnpm-lock.yaml"); err != nil {
		return nil, err
	} else if !detected {
		return nil, ErrNoBuilder
	}

	name := filepath.Base(wd)
	return map[string]Interface{name: b}, nil
}

func (PNPMBasic) Build(eng Engine, unit *BuildUnit) (container *dagger.Container, err error) {
	defer errutil.Wrap("pnpm/basic", &err)

	host := eng.Client().Host().
		Directory(unit.WorkDir, dagger.HostDirectoryOpts{Exclude: unit.Excludes})

	// prepare job parameters
	outdir := strings.TrimSpace(unit.BuildDir)
	if outdir == "" {
		outdir = "build"
	}

	cmd := strings.TrimSpace(unit.CommandName)
	if cmd == "" {
		cmd = "/usr/local/bin/node"
	}

	args := []string{cmd}
	if len(unit.CommandArgs) > 0 {
		args = append(args, unit.CommandArgs...)
	} else {
		args = append(args, ".")
	}

	// build
	base := BaseImageForUnit(eng, unit)
	base = withPNPMBase(base)
	base = withPNPMPkgCache(eng, base)
	base = withUnitEnv(base, unit)
	base = base.
		WithFile("package.json", host.File("package.json")).
		WithFile("pnpm-lock.yaml", host.File("pnpm-lock.yaml")).
		WithExec([]string{"pnpm", "i"})

	builder := base.
		WithDirectory("/app", host).
		WithExec([]string{"pnpm", "build"})

	// runner
	runner := withRunnerPkgs(base).
		WithDirectory("/app", builder.Directory(outdir))
	for _, dir := range unit.AssetDirs {
		runner = runner.WithDirectory(dir, builder.Directory(dir))
	}

	runner = runner.WithDefaultArgs(args)
	return runner.Sync(eng.Context())
}
