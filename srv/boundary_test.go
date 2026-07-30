package srv_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// serverConcerns are the imports that make a package part of the server. The shared
// build/render/publish packages are the leaves of this repo and must reach none of them: a
// CLI binary that dragged in a database driver and an HTTP server would pay for a server it
// never starts, and a shared package that knew srv existed could not stay reusable by it.
var serverConcerns = []string{
	"platform.prodigy9.co/srv",
	"fx.prodigy9.co/data",
	"fx.prodigy9.co/httpserver",
	"github.com/jmoiron/sqlx",
	"github.com/go-chi/chi",
}

// TestSharedPackagesNeverImportServerConcerns guards the one rule that crosses from srv into
// the rest of the repo. It reads the real import graph rather than the source, so a concern
// arriving two hops down some new dependency is caught as surely as a direct import.
func TestSharedPackagesNeverImportServerConcerns(t *testing.T) {
	for _, pkg := range sharedPackages(t) {
		for _, dep := range dependenciesOf(t, pkg) {
			for _, concern := range serverConcerns {
				require.False(t, strings.HasPrefix(dep, concern),
					"%s reaches %s: shared packages are leaves and never import server concerns", pkg, dep)
			}
		}
	}
}

// sharedPackages is every package outside the server, the CLI and the embedded UI — the
// leaves the rule binds. It is derived rather than listed so a new shared package is covered
// the moment it exists.
func sharedPackages(t *testing.T) []string {
	// The module root is the main package and is excluded by name; the three layers above
	// the leaves are excluded with everything beneath them.
	const mainPkg = "platform.prodigy9.co"
	notShared := []string{
		"platform.prodigy9.co/srv",
		"platform.prodigy9.co/cmd",
		"platform.prodigy9.co/webui",
	}

	// The pattern is module-rooted, not ./...: a test runs in its own package's directory,
	// where ./... would only ever list srv itself.
	shared := []string{}
	for _, pkg := range goList(t, mainPkg+"/...") {
		if pkg != mainPkg && !isUnder(pkg, notShared) {
			shared = append(shared, pkg)
		}
	}

	require.NotEmpty(t, shared, "the rule is vacuous if it guards nothing")
	return shared
}

func dependenciesOf(t *testing.T, pkg string) []string {
	return goList(t, "-deps", pkg)
}

func goList(t *testing.T, args ...string) []string {
	out, err := exec.Command("go", append([]string{"list"}, args...)...).Output()
	require.NoError(t, err, "go list %v", args)

	return strings.Fields(string(out))
}

// isUnder reports whether pkg is one of the prefixes or lives beneath one, so a prefix
// covers a whole subtree without matching a package that merely starts with the same letters.
func isUnder(pkg string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if pkg == prefix || strings.HasPrefix(pkg, prefix+"/") {
			return true
		}
	}
	return false
}
