package framework

import (
	"context"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/framework/scaffold"
	"platform.prodigy9.co/internal/buildlog"
)

type Dockerfile struct{ noScaffoldInputs }

var _ Framework = Dockerfile{}

func (Dockerfile) Name() string   { return "dockerfile" }
func (Dockerfile) Layout() Layout { return LayoutBasic }

func (Dockerfile) Discover(wd string) bool {
	detected, _ := detectFile(wd, "Dockerfile")
	return detected
}

func (fw Dockerfile) Scaffold(ctx context.Context, wd, _, _ string, _ map[string]string) (scaffold.Spec, error) {
	return scaffold.Spec{Module: defaultModule(fw, wd)}, nil
}

func (Dockerfile) Build(ctx context.Context, client *dagger.Client, unit *BuildUnit) (container *dagger.Container, err error) {
	defer errutil.Wrap("dockerfile", &err)

	buildlog.Logger().Warn("dockerfile framework bypasses the Wolfi base image and platform package conventions; prefer a language-specific framework (go/basic, go/workspace, pnpm/basic, pnpm/static, pnpm/workspace) when possible",
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

	// not using BaseImageForUnit because, well, dockerfiles have their own bases
	// this framework should be discouraged
	opts := dagger.DirectoryDockerBuildOpts{
		Platform: dagger.Platform(unit.Arch),
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
