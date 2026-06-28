package builder

import (
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/builder/fileutil"
)

type PNPMStatic struct{}

func (PNPMStatic) Name() string   { return "pnpm/static" }
func (PNPMStatic) Layout() Layout { return LayoutBasic }
func (PNPMStatic) Class() Class   { return ClassInterpreted }

func (b PNPMStatic) Discover(wd string) (map[string]Interface, error) {
	// Assumes astro = static site, for now.
	if detected, err := fileutil.DetectFile(wd, "astro.config.mjs"); err != nil {
		return nil, err
	} else if !detected {
		return nil, ErrNoBuilder
	}

	name := filepath.Base(wd)
	return map[string]Interface{name: b}, nil
}

func (b PNPMStatic) Build(eng Engine, unit *BuildUnit) (container *dagger.Container, err error) {
	defer errutil.Wrap("pnpm/static", &err)

	host := eng.Client().Host().
		Directory(unit.WorkDir, dagger.HostDirectoryOpts{Exclude: unit.Excludes})

	builder := BaseImageForUnit(eng, unit)
	builder = withPNPMBase(builder)
	builder = withPNPMPkgCache(eng, builder)

	builder = builder.
		WithFile("package.json", host.File("package.json")).
		WithFile("pnpm-lock.yaml", host.File("pnpm-lock.yaml")).
		WithExec([]string{"pnpm", "i"}).
		WithDirectory("/app", host).
		WithExec([]string{"pnpm", "build"})

	outdir := strings.TrimSpace(unit.BuildDir)
	if outdir == "" {
		outdir = "build"
	}

	cmd := strings.TrimSpace(unit.CommandName)
	if cmd == "" {
		cmd = "caddy"
	}

	args := []string{cmd}
	if len(unit.CommandArgs) > 0 {
		args = append(args, unit.CommandArgs...)
	} else {
		args = append(args, "file-server", "-l", "0.0.0.0:3000")
	}

	runner := BaseImageForUnit(eng, unit)
	runner = withCaddyServer(runner).
		WithDirectory("/app", builder.Directory(outdir)).
		WithDefaultArgs(args)

	return runner, nil
}
