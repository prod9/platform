package framework

import (
	"context"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/framework/scaffold"
	"platform.prodigy9.co/internal/buildlog"
)

type Dockerfile struct{ noScaffoldVars }

var _ Framework = Dockerfile{}

func (Dockerfile) Name() string   { return "dockerfile" }
func (Dockerfile) Layout() Layout { return LayoutBasic }

func (Dockerfile) Discover(wd string) bool {
	return hasFile(wd, "Dockerfile")
}

func (fw Dockerfile) Scaffold(ctx context.Context, wd string, _ scaffold.Env, _ map[string]string) (scaffold.Spec, error) {
	return scaffold.Spec{Module: defaultModule(fw, wd)}, nil
}

// Plan is one step: the user's Dockerfile is opaque to us, so there is nothing to cut it
// into. Its whole build is one silence, which is part of why the framework is discouraged.
func (Dockerfile) Plan(*BuildUnit) []Step {
	return []Step{StepBuild}
}

func (Dockerfile) Execute(ctx context.Context, client *dagger.Client, unit *BuildUnit, step Step, _ *dagger.Container) (container *dagger.Container, err error) {
	defer errutil.Wrap("dockerfile", &err)

	if step != StepBuild {
		return nil, unknownStep(step)
	}

	buildlog.Logger().Warn("dockerfile framework bypasses the Wolfi base image and platform package conventions; prefer a language-specific framework (go/basic, go/workspace, pnpm/basic, pnpm/static, pnpm/workspace) when possible",
		"module", unit.Name,
		"workdir", unit.WorkDir,
	)

	host := unitHost(client, unit)

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

	return builder, nil
}
