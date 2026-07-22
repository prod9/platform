package framework

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/framework/scaffold"
)

var (
	ErrBadFramework = errors.New("framework: invalid framework")
	ErrNoFramework  = errors.New("framework: no compatible framework detected")
)

type (
	Layout string

	// Framework is the sole owner of a project type: it recognizes itself (Discover),
	// scaffolds itself (Scaffold), and builds itself (Build). A framework is a stateless
	// value carrying per-stack knowledge and nothing else.
	Framework interface {
		Name() string
		Layout() Layout

		// Discover reports whether this framework owns wd. Scaffold-time only — the
		// build path resolves the framework by the [modules] name, never re-discovers.
		Discover(wd string) bool

		// ScaffoldVars lists the operator inputs this framework needs at init,
		// by name (the name is the prompt label). The driver prompts each and passes the
		// answers back via ScaffoldData. Most frameworks onboard an existing repo and need
		// none (nil); PlatformInfra needs the CUE module path only when greenfield.
		ScaffoldVars(wd string) []string

		// Scaffold returns the framework's full, ready-to-write contribution to a fresh repo:
		// its platform.toml module, default [vars], the strategy it seeds, and the files it
		// ships with every template hole already resolved. The framework owns resolution — it
		// knows which operator input fills which hole (e.g. CUE_MOD_PREFIX -> the CUE module
		// path) and how to read existing state (an existing cue.mod wins over the input).
		// repository and daggerVersion are environment facts the driver supplies; inputs are
		// the operator's answers to ScaffoldVars. The driver just writes what it gets.
		Scaffold(ctx context.Context, wd string, env scaffold.Env, inputs map[string]string) (scaffold.Spec, error)

		Build(ctx context.Context, client *dagger.Client, unit *BuildUnit) (*dagger.Container, error)
	}
)

// noScaffoldVars is the default for frameworks that onboard an existing repo: they read
// their own module file (go.mod, package.json) rather than scaffolding one, so they need no
// operator inputs. Embed it to satisfy ScaffoldVars.
type noScaffoldVars struct{}

func (noScaffoldVars) ScaffoldVars(string) []string { return nil }

const (
	LayoutBasic     Layout = "basic"
	LayoutWorkspace Layout = "workspace"
)

var (
	// IMPORTANT: This list is **Order Sensitive** due to Discover() calls on different
	// frameworks discovering the same subfolder a little differently.
	knownFrameworks = []Framework{
		PlatformInfra{},
		GoWorkspace{},
		PNPMWorkspace{},
		GoBasic{},
		PNPMStatic{},
		PNPMBasic{},
		Dockerfile{},
	}
)

func FindFramework(name string) (Framework, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	for _, fw := range knownFrameworks {
		if fw.Name() == name {
			return fw, nil
		}
	}

	return nil, fmt.Errorf("%s: %w", name, ErrBadFramework)
}

func Discover(wd string) (Framework, error) {
	for _, fw := range knownFrameworks {
		if fw.Discover(wd) {
			return fw, nil
		}
	}
	return nil, ErrNoFramework
}

// hasFile is the discovery probe: reports whether wd carries the framework's marker
// file (go.mod, pnpm-lock.yaml, Dockerfile, …). A stat that fails for any reason means
// the marker is not usable, which is indistinguishable from absent to a Discover.
func hasFile(wd, filename string) bool {
	_, err := os.Stat(filepath.Join(wd, filename))
	return err == nil
}

// defaultModule is the single-module platform.toml contribution shared by the
// frameworks, with WorkDir set per layout (workspace layouts nest the module under
// ./<name>, basic ones sit at the root). The driver keys it by the directory name.
func defaultModule(fw Framework, wd string) *conf.Module {
	mod := *conf.ModuleDefaults
	mod.Framework = fw.Name()
	if fw.Layout() == LayoutWorkspace {
		mod.WorkDir = "./" + filepath.Base(wd)
	} else {
		mod.WorkDir = "."
	}
	return &mod
}
