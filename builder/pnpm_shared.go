package builder

import "dagger.io/dagger"

const NInstallScript = `
curl -fsSL https://raw.githubusercontent.com/tj/n/master/bin/n | \
	bash -s install lts
`

func withPNPMBase(base *dagger.Container) *dagger.Container {
	return base.
		WithNewFile("/install-n.sh", NInstallScript).
		WithExec([]string{"/usr/bin/bash", "/install-n.sh"}).
		WithExec([]string{"corepack", "enable", "pnpm"}).
		WithExec([]string{"corepack", "install", "-g", "pnpm"})
}

func withTypeModulePackageJSON(base *dagger.Container) *dagger.Container {
	return base.WithNewFile("/app/package.json", `{"type":"module"}`)
}

func withPNPMPkgCache(sess *Session, base *dagger.Container) *dagger.Container {
	cache := sess.Client().CacheVolume("platform-pnpm-cache")
	return base.WithMountedCache("/root/.local/share/pnpm", cache)
}
