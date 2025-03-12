package builder

import (
	"errors"
	"fmt"
	"strings"

	"dagger.io/dagger"
	fxconfig "fx.prodigy9.co/config"
	"platform.prodigy9.co/internal"
	"platform.prodigy9.co/internal/plog"
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

type (
	Layout string
	Class  string

	Interface interface {
		Name() string
		Layout() Layout
		Class() Class

		Discover(wd string) (map[string]Interface, error)
		Build(sess *Session, job *Job) (*dagger.Container, error)
	}

	BuildResult struct {
		Job       *Job
		Container *dagger.Container
		Err       error
	}

	PublishResult struct {
		BuildResult
		ImageName string
		ImageHash string
	}
)

const (
	LayoutBasic     Layout = "basic"
	LayoutWorkspace Layout = "workspace"

	// ClassNative specifies that the builder produces machine-native binary that can be
	// directly executed without any additional VMs or interpreter required inside built
	// container.
	//
	// This means the builder has a compilation step and the compilation result can be used
	// directly.
	//
	// Examples: Go, Rust
	ClassNative Class = "native"

	// ClassBytecode specifies that the builder produces a binary file that are not
	// machine-native and requires the use of an additional VM or runtime setup inside built
	// container.
	//
	// This means the builder has a compilation step and the compilation result requires a
	// VM or runtime to run.
	//
	// Examples: Java, Erlang, Elixir
	ClassBytecode Class = "bytecode"

	// ClassInterpreted specifies that the builder does not produce a binary file, instead
	// it outputs a compressed/minified/bundled/packaged version of the source files.
	//
	// This means the builder does not have a compilation step and it simply processes
	// source files into a more production-ready forms and usually requires the same
	// toolings to be installed during buildtime and runtime.
	//
	// Examples: Ruby on Rails, Node.js
	ClassInterpreted Class = "interpreted"

	// ClassCustom specifies that the builder has its own heavily customized build process
	// that cannot be easily categorized or genericized into the other classes.
	//
	// Examples: Dockerfile
	ClassCustom Class = "custom"
)

var (
	// IMPORTANT: This list is **Order Sensitive** due to Discover() calls on different
	// builders discovering the same subfolder a little differently.
	knownBuilders = []Interface{
		GoWorkspace{},
		PNPMWorkspace{},
		GoBasic{},
		PNPMStatic{},
		PNPMBasic{},
		Dockerfile{},
	}
)

func All() []Interface { return knownBuilders }

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

func Build(sess *Session, jobs ...*Job) ([]BuildResult, error) {
	if len(jobs) == 0 {
		return nil, ErrNoJobs
	}

	m := &internal.Multiplexer[*Job, BuildResult]{}
	m.Reset(jobs)
	return m.Start(func(idx int, job *Job) BuildResult {
		ctx, cancel := sess.JobContext(job)
		defer cancel()

		container, err := job.Builder.Build(sess, job)
		if err != nil {
			return BuildResult{Job: job, Container: nil, Err: err}
		}

		container, err = container.Sync(ctx)
		if err != nil {
			return BuildResult{Job: job, Container: nil, Err: err}
		} else {
			return BuildResult{Job: job, Container: container, Err: nil}
		}
	}), nil
}

func Publish(sess *Session, builds ...BuildResult) ([]PublishResult, error) {
	if len(builds) == 0 {
		return nil, ErrNoJobs
	}

	fxcfg := fxconfig.Configure()
	registryPassword := sess.Client().SetSecret(
		RegistryPasswordConfig.Name(),
		fxconfig.Get(fxcfg, RegistryPasswordConfig),
	)

	m := &internal.Multiplexer[BuildResult, PublishResult]{}
	m.Reset(builds)
	return m.Start(func(idx int, build BuildResult) PublishResult {
		if build.Err != nil {
			return PublishResult{BuildResult: build}
		}

		container := build.Container
		if fxconfig.Get(fxcfg, RegistryUsernameConfig) != "" {
			container = container.WithRegistryAuth(
				fxconfig.Get(fxcfg, RegistryConfig),
				fxconfig.Get(fxcfg, RegistryUsernameConfig),
				registryPassword,
			)
		}

		hash, err := container.Publish(sess.Context(), build.Job.ImageName)
		if err != nil {
			build.Err = err
			return PublishResult{BuildResult: build}
		}

		plog.Image("publish", build.Job.ImageName, hash)
		return PublishResult{
			BuildResult: build,
			ImageName:   build.Job.ImageName,
			ImageHash:   hash,
		}
	}), nil
}
