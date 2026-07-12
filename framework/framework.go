package framework

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"dagger.io/dagger"
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

		// RequiredScaffoldInputs lists the operator inputs this framework needs at init,
		// by name (the name is the prompt label). The driver prompts each and passes the
		// answers back via ScaffoldData. Most frameworks onboard an existing repo and need
		// none (nil); Infra needs the CUE module path only when greenfield.
		RequiredScaffoldInputs(wd string) []string

		// Scaffold returns the framework's full declarative contribution to a fresh
		// repo: its platform.toml module, default [vars], the files it ships (holes
		// unresolved), and the strategy value it seeds. Pure — the driver resolves and writes.
		Scaffold(ctx context.Context, wd string) (scaffold.Spec, error)

		// ScaffoldData builds the values that fill the Scaffold files' template holes from
		// the operator inputs — the framework owns which input maps to which hole (e.g.
		// CUE_MOD_PREFIX -> the CUE module path) and how to read existing state (an existing
		// cue.mod wins over the input). repository and daggerVersion are environment facts
		// the driver supplies. Frameworks that ship no template files return the zero Data.
		ScaffoldData(wd, repository, daggerVersion string, inputs map[string]string) (scaffold.Data, error)

		Build(ctx context.Context, client *dagger.Client, unit *BuildUnit) (*dagger.Container, error)
	}
)

// noScaffoldInputs is the default for frameworks that onboard an existing repo: they read
// their own module file (go.mod, package.json) rather than scaffolding one, so they need no
// operator inputs and contribute no template data. Embed it to satisfy the contract.
type noScaffoldInputs struct{}

func (noScaffoldInputs) RequiredScaffoldInputs(string) []string { return nil }

func (noScaffoldInputs) ScaffoldData(_, _, _ string, _ map[string]string) (scaffold.Data, error) {
	return scaffold.Data{}, nil
}

const (
	LayoutBasic     Layout = "basic"
	LayoutWorkspace Layout = "workspace"
)

var (
	// IMPORTANT: This list is **Order Sensitive** due to Discover() calls on different
	// frameworks discovering the same subfolder a little differently.
	knownFrameworks = []Framework{
		Infra{},
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
