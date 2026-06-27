package builder

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

func withTypeModulePackageJSON(base *dagger.Container) *dagger.Container {
	return base.WithNewFile("/app/package.json", `{"type":"module"}`)
}

func withPNPMPkgCache(sess Engine, base *dagger.Container) *dagger.Container {
	cache := sess.Client().CacheVolume("platform-pnpm-cache")
	return base.WithMountedCache("/root/.local/share/pnpm", cache)
}
