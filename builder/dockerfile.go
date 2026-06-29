package builder

import (
	"context"
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/builder/fileutil"
	"platform.prodigy9.co/internal/buildlog"
)

type Dockerfile struct{}

var _ Interface = Dockerfile{}

func (d Dockerfile) Name() string   { return "dockerfile" }
func (d Dockerfile) Layout() Layout { return LayoutBasic }
func (d Dockerfile) Class() Class   { return ClassCustom }

func (d Dockerfile) Discover(wd string) (map[string]Interface, error) {
	if detected, err := fileutil.DetectFile(wd, "Dockerfile"); err != nil {
		return nil, err
	} else if !detected {
		return nil, ErrNoBuilder
	}

	name := filepath.Base(wd)
	return map[string]Interface{name: d}, nil
}

func (d Dockerfile) Build(ctx context.Context, client *dagger.Client, unit *BuildUnit) (container *dagger.Container, err error) {
	defer errutil.Wrap("dockerfile", &err)

	buildlog.Logger().Warn("dockerfile builder bypasses the Wolfi base image and platform package conventions; prefer a language-specific builder (go/basic, go/workspace, pnpm/basic, pnpm/static, pnpm/workspace) when possible",
		"module", unit.Name,
		"workdir", unit.WorkDir,
	)

	host := client.Host().Directory(unit.WorkDir, dagger.HostDirectoryOpts{
		Exclude: unit.Excludes,
	})

	cmd := strings.TrimSpace(unit.CommandName)
	var args []string
	if cmd != "" {
		args = append(args, cmd)
	}
	if len(unit.CommandArgs) > 0 {
		args = append(args, unit.CommandArgs...)
	}

	// not using BaseImageForJob because, well, dockerfiles have their own bases
	// this builder should be discouraged
	opts := dagger.DirectoryDockerBuildOpts{
		Platform: dagger.Platform(unit.Platform),
	}
	for key, value := range unit.Env {
		opts.BuildArgs = append(opts.BuildArgs,
			dagger.BuildArg{Name: key, Value: value},
		)
	}

	builder := host.DockerBuild(opts)

	builder = withUnitEnv(builder, unit)
	if len(args) > 0 {
		builder = builder.WithDefaultArgs(args)
	}

	return builder.Sync(ctx)
}
