package builder

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"dagger.io/dagger"
)

var (
	ErrBadBuilder = errors.New("builder: invalid builder")
	ErrNoBuilder  = errors.New("builder: no compatible builder detected")
)

type (
	Layout string
	Class  string

	Interface interface {
		Name() string
		Layout() Layout
		Class() Class

		Discover(wd string) (map[string]Interface, error)
		Build(ctx context.Context, client *dagger.Client, unit *BuildUnit) (*dagger.Container, error)
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

	// ClassStatic specifies that the builder produces a set of static assets served
	// directly by a webserver, with no language runtime or interpreter present in the
	// runtime container. The build toolchain and the runtime server are fully decoupled,
	// so the build language is incidental to the class.
	//
	// Examples: Astro, Hugo, plain HTML
	ClassStatic Class = "static"

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
