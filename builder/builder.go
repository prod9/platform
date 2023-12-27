package builder

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"dagger.io/dagger"
	fxconfig "fx.prodigy9.co/config"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/internal/plog"
	"platform.prodigy9.co/project"
)

var (
	ErrBadBuilder = errors.New("builder: invalid builder")
	ErrNoBuilder  = errors.New("builder: no compatible builder detected")
	ErrNoJobs     = errors.New("builder: empty jobs list, nothing to do")
)

// non-standard project settings using fx's env variable configuration. These are meant to
// be set in CI agents so that the credentials do not need to be stored with each
// project's codebase.
var (
	RegistryConfig         = fxconfig.Str("REGISTRY")
	RegistryUsernameConfig = fxconfig.Str("REGISTRY_USERNAME")
	RegistryPasswordConfig = fxconfig.Str("REGISTRY_PASSWORD")
)

type Kind string

const (
	KindBasic     Kind = "basic"
	KindWorkspace Kind = "workspace"
)

type Interface interface {
	Name() string
	Kind() Kind

	Discover(wd string) (map[string]Interface, error)
	Build(ctx context.Context, client *dagger.Client, job *Job) (*dagger.Container, error)
}

var (
	knownBuilders = []Interface{
		// Order sensitive due to Discover() calls on different builders discovering the same
		// subfolder a little differently.
		GoWorkspace{},
		PNPMWorkspace{},
		GoBasic{},
		PNPMBasic{},
	}

	basicBuilders     []Interface
	workspaceBuilders []Interface
)

func init() {
	for _, builder := range knownBuilders {
		switch builder.Kind() {
		case KindBasic:
			basicBuilders = append(basicBuilders, builder)
		case KindWorkspace:
			workspaceBuilders = append(workspaceBuilders, builder)
		}
	}
}

func All() []Interface        { return knownBuilders }
func Basics() []Interface     { return basicBuilders }
func Workspaces() []Interface { return workspaceBuilders }

func FindBuilder(name string) (Interface, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	for _, builder := range knownBuilders {
		if builder.Name() == name {
			return builder, nil
		}
	}

	return nil, fmt.Errorf("%s: %w", name, ErrBadBuilder)
}

func Discover(wd string) (map[string]Interface, error) {
	for _, builder := range knownBuilders {
		if mods, err := builder.Discover(wd); errors.Is(err, ErrNoBuilder) {
			continue
		} else if err != nil {
			return nil, err
		} else if len(mods) > 0 {
			return mods, nil
		}
	}
	return nil, ErrNoBuilder
}

// TODO: Build should return a JobResult or something so we can add more manipulation
// commands after image is built
func Build(cfg *project.Project, jobs ...*Job) error {
	if len(jobs) == 0 {
		return ErrNoJobs
	}

	ctx := context.Background()
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(plog.OutputForDagger()))
	if err != nil {
		return err
	}
	defer client.Close()

	fxcfg := fxconfig.Configure()
	registryPassword := client.SetSecret(
		RegistryPasswordConfig.Name(),
		fxconfig.Get(fxcfg, RegistryPasswordConfig),
	)

	return errutil.AggregateWithTags(jobs, func(idx int, job *Job) (string, error) {
		ctx, cancel := context.WithTimeout(ctx, job.Timeout)
		defer cancel()

		container, err := job.Builder.Build(ctx, client, job)
		if err != nil {
			return job.Name, err
		} else if container, err = container.Sync(ctx); err != nil {
			return job.Name, err
		}

		if job.Publish {
			if fxconfig.Get(fxcfg, RegistryUsernameConfig) != "" {
				container = container.WithRegistryAuth(
					fxconfig.Get(fxcfg, RegistryConfig),
					fxconfig.Get(fxcfg, RegistryUsernameConfig),
					registryPassword,
				)
			}

			hash, err := container.Publish(ctx, job.ImageName)
			if err != nil {
				return job.Name, err
			}
			plog.Image(hash)
		}

		return job.Name, nil
	})
}
