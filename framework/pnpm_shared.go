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
func withPNPMModuleFix(base *dagger.Container) *dagger.Container {
	return base.WithNewFile(RunDir+"/package.json", `{"type":"module"}`)
}

func withPNPMPkgCache(client *dagger.Client, base *dagger.Container) *dagger.Container {
	cache := client.CacheVolume("platform-pnpm-cache")
	return base.WithMountedCache("/root/.local/share/pnpm", cache)
}
