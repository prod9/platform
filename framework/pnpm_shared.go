package framework

import (
	"strings"

	"dagger.io/dagger"
)

const (
	defaultBuildDir = "build"               // pnpm output dir when BuildDir is unset
	defaultNodeBin  = "/usr/local/bin/node" // run command for non-static pnpm builds
)

// nInstallScript installs the Node runtime pnpm rides on, from nodejs.org via tj/n.
var nInstallScript = strings.TrimSpace(`
set -xe
curl -fsSL https://raw.githubusercontent.com/tj/n/master/bin/n | \
	bash -s install lts
`)

// withPNPM provisions pnpm — the runtime beneath it via tj/n, then corepack, which is how
// pnpm itself arrives. Neither version is ours to name: `lts` is a moving target on purpose,
// and corepack resolves pnpm from the repo's own packageManager field — a repo that declares
// none is not built.
func withPNPM(base *dagger.Container) *dagger.Container {
	return base.
		WithNewFile("/install-n.sh", nInstallScript).
		WithExec([]string{"/usr/bin/bash", "/install-n.sh"}).
		WithExec([]string{"corepack", "enable", "pnpm"})
}

// withPNPMPkgCache mounts the persistent pnpm store so package pulls survive across builds.
func withPNPMPkgCache(client *dagger.Client, base *dagger.Container) *dagger.Container {
	cache := client.CacheVolume("platform-pnpm-cache")
	return base.WithMountedCache("/root/.local/share/pnpm", cache)
}

// withPNPMDeps installs from the manifests alone, copied ahead of the source so the layer
// keys on them and survives every source edit. The include filter copies whichever the repo
// actually has, so a project with no pnpm-workspace.yaml is unaffected — and that file must
// be in the list, because from pnpm v10 it holds every non-auth setting, including the
// allowBuilds approvals that let a dependency run its install scripts. Drop it and the repo's
// committed approvals never reach the container, so those dependencies silently go unbuilt.
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

// withPNPMModuleFix marks the runner's served directory as ESM so bare node treats the
// pnpm/workspace output as modules. pnpm-specific — no other family needs it.
func withPNPMModuleFix(base *dagger.Container) *dagger.Container {
	return base.WithNewFile(RunDir+"/package.json", `{"type":"module"}`)
}

// pnpmRunArgs builds a pnpm runner's default args: the resolved command followed by
// the operator's CommandArgs, or the framework's fallback args when none are given.
func pnpmRunArgs(cmd string, unit *BuildUnit, fallback ...string) []string {
	args := []string{cmd}
	if len(unit.CommandArgs) > 0 {
		return append(args, unit.CommandArgs...)
	}
	return append(args, fallback...)
}
