package framework

import (
	"strings"

	"dagger.io/dagger"
)

// Pinned versions so smoke tests and production builds don't drift with upstream.
// Bump deliberately and re-run `./test.sh` to confirm.
const (
	NodeVersion = "22"     // Node.js LTS line
	PNPMVersion = "9.15.5" // pnpm 10+ turns ignored-build warnings into errors
)

const (
	defaultBuildDir = "build"               // pnpm output dir when BuildDir is unset
	defaultNodeBin  = "/usr/local/bin/node" // run command for non-static pnpm builds
)

// pnpmRunArgs builds a pnpm runner's default args: the resolved command followed by
// the operator's CommandArgs, or the framework's fallback args when none are given.
func pnpmRunArgs(cmd string, unit *BuildUnit, fallback ...string) []string {
	args := []string{cmd}
	if len(unit.CommandArgs) > 0 {
		return append(args, unit.CommandArgs...)
	}
	return append(args, fallback...)
}

var NInstallScript = strings.TrimSpace(`
set -xe
curl -fsSL https://raw.githubusercontent.com/tj/n/master/bin/n | \
	bash -s install ` + NodeVersion + `
`)

func withPNPMBase(base *dagger.Container) *dagger.Container {
	return withBuildPkgs(base).
		WithNewFile("/install-n.sh", NInstallScript).
		WithExec([]string{"/usr/bin/bash", "/install-n.sh"}).
		WithExec([]string{"corepack", "enable", "pnpm"}).
		WithExec([]string{"corepack", "install", "-g", "pnpm@" + PNPMVersion})
}

// withPNPMModuleFix marks the runner's served directory as ESM so bare node treats
// the pnpm/workspace output as modules. pnpm-specific — no other family needs it.
// pnpmDepManifests are the files the dependency layer installs from, copied ahead of the
// source so the layer keys on manifests alone. pnpm-workspace.yaml belongs here because
// from pnpm v10 it holds every non-auth setting — including the allowBuilds approvals that
// let a dependency run its install scripts. Drop it and the repo's committed approvals
// never reach the container, so those dependencies silently go unbuilt.
var pnpmDepManifests = []string{"package.json", "pnpm-lock.yaml", "pnpm-workspace.yaml"}

// withPNPMDeps installs from the manifests alone. The include filter copies whichever of
// them the repo actually has, so a project with no pnpm-workspace.yaml is unaffected.
func withPNPMDeps(base *dagger.Container, host *dagger.Directory) *dagger.Container {
	return base.
		WithWorkdir(SrcDir).
		WithDirectory(".", host, dagger.ContainerWithDirectoryOpts{Include: pnpmDepManifests}).
		WithExec([]string{"pnpm", "i"})
}

func withPNPMModuleFix(base *dagger.Container) *dagger.Container {
	return base.WithNewFile(RunDir+"/package.json", `{"type":"module"}`)
}

func withPNPMPkgCache(client *dagger.Client, base *dagger.Container) *dagger.Container {
	cache := client.CacheVolume("platform-pnpm-cache")
	return base.WithMountedCache("/root/.local/share/pnpm", cache)
}
