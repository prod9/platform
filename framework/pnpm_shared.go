package framework

import (
	"strings"

	"dagger.io/dagger"
)

const (
	// sveltekitBuildDir is what SvelteKit's adapter-node defaults its `out` to, and
	// pnpm/basic discovers SvelteKit projects. `pnpm/static` is Astro and defaults
	// elsewhere — see astroBuildDir.
	sveltekitBuildDir = "build"
	defaultNodeBin    = "/usr/local/bin/node" // run command for non-static pnpm builds
)

// nInstallScript installs the Node runtime pnpm rides on, from nodejs.org via tj/n.
var nInstallScript = strings.TrimSpace(`
set -xe
curl -fsSL https://raw.githubusercontent.com/tj/n/master/bin/n | \
	bash -s install lts
`)

// withPNPM provisions pnpm — the runtime beneath it via tj/n, then corepack, which is how
// pnpm itself arrives. No version is ours to name: `lts` is a moving target on purpose,
// corepack follows its own latest, and corepack resolves pnpm from the repo's own
// packageManager field — a repo that declares none is not built. Node 25 and later ship
// no corepack, so it comes from npm rather than with Node (docs/spec/frameworks.md).
func withPNPM(base *dagger.Container) *dagger.Container {
	return base.
		WithNewFile("/install-n.sh", nInstallScript).
		WithExec([]string{"/usr/bin/bash", "/install-n.sh"}).
		WithExec([]string{"npm", "install", "--global", "corepack"}).
		WithExec([]string{"corepack", "enable", "pnpm"})
}

// withPNPMPkgCache mounts the persistent pnpm store so pulls survive across builds.
func withPNPMPkgCache(client *dagger.Client, base *dagger.Container) *dagger.Container {
	cache := client.CacheVolume("platform-pnpm-cache")
	return base.WithMountedCache("/root/.local/share/pnpm", cache)
}

// withPNPMDeps installs from the manifests alone, copied ahead of the source so the layer
// keys on them and survives every source edit. The include filter copies whichever the
// repo actually has, so a project with no pnpm-workspace.yaml is unaffected — and that
// file must be in the list, because from pnpm v10 it holds every non-auth setting,
// including the allowBuilds approvals that let a dependency run its install scripts. Drop
// it and the repo's committed approvals never reach the container, so those dependencies
// silently go unbuilt.
func withPNPMDeps(base *dagger.Container, host *dagger.Directory) *dagger.Container {
	return base.
		WithWorkdir(SrcDir).
		WithDirectory(".", host, dagger.ContainerWithDirectoryOpts{
			Include: []string{
				"package.json",
				"pnpm-lock.yaml",
				"pnpm-workspace.yaml",
			},
		}).
		WithExec([]string{"pnpm", "i"})
}
