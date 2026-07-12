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

		// Scaffold returns the framework's full declarative contribution to a fresh
		// repo: its platform.toml module, default [vars], the files it ships (holes
		// unresolved), the strategy value it seeds, and whether it needs a
		// freshly-created git repo. Pure — the driver resolves and writes.
		Scaffold(ctx context.Context, wd string) (scaffold.Spec, error)

		Build(ctx context.Context, client *dagger.Client, unit *BuildUnit) (*dagger.Container, error)
	}
)

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
