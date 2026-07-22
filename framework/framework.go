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
	ErrUnknownStep  = errors.New("framework: unknown step")
)

type (
	Layout string

	// Step is an opaque label for one stage of a unit's build — no payload, no closure.
	// Plan returns the ordered steps; Execute switches on the one it is handed.
	//
	// Step granularity is log granularity: a step is the unit the engine times and, once
	// build events land, the unit whose output is flushed. A long step is therefore a long
	// silence with no remedy short of cutting it up, which makes "every time-taking stage
	// earns its own Step" a constraint on how Plan is authored, not a preference.
	Step string

	// Framework is the sole owner of a project type: it recognizes itself (Discover),
	// scaffolds itself (Scaffold), and knows how it is built (Plan + Execute). A framework
	// is a stateless value carrying per-stack knowledge and nothing else.
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

		// Plan returns the ordered steps this unit's build is made of. It is pure — the
		// framework stores nothing between calls, so a plan can be recomputed at any time
		// and the engine always drives the latest one.
		Plan(unit *BuildUnit) []Step

		// Execute runs exactly one step: it receives the previous step's container and
		// returns the next one. The first step receives nil and establishes the base. The
		// container is the chaining medium between steps, not an obligation — a step that
		// does host-side work only (PlatformInfra's dep fetch) passes its input straight
		// through. How a step does its work is entirely the framework's business; the
		// engine imposes the sequence and nothing else.
		Execute(ctx context.Context, client *dagger.Client, unit *BuildUnit, step Step, in *dagger.Container) (*dagger.Container, error)
	}
)

// The shared step vocabulary. Frameworks reuse these labels so an operator reads the same
// stage names across stacks; a framework with a stage none of these name is free to
// declare its own.
const (
	StepBase   Step = "base"   // establish the base image
	StepDeps   Step = "deps"   // fetch this stack's dependencies
	StepTest   Step = "test"   // run the module's tests (a hard gate — a red suite fails the build)
	StepBuild  Step = "build"  // run the stack's own build command
	StepRunner Step = "runner" // assemble the runtime image from the build's output
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
	// IMPORTANT: This list is **Order Sensitive** — Discover() is first-match-wins and
	// one directory satisfies several markers at once (a pnpm workspace root also has
	// pnpm-lock.yaml; any repo may carry a Dockerfile). It runs narrowest marker first,
	// broadest last; the group markers below say what each tier matches on. Inserting a
	// framework into the wrong group silently re-routes existing projects, so place it by
	// how specific its marker is — never alphabetically.
	knownFrameworks = []Framework{
		// directory name, not a file: base(wd) contains "infra"
		PlatformInfra{},

		// workspace roots — their members match the single-project markers below
		GoWorkspace{},   // go.work
		PNPMWorkspace{}, // pnpm-workspace.yaml, pnpm-workspaces.yaml

		// single-project markers
		GoBasic{},    // go.mod
		PNPMStatic{}, // astro.config.mjs

		// broadest — present in every project of their kind
		PNPMBasic{},  // pnpm-lock.yaml
		Dockerfile{}, // Dockerfile, in any repo
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
