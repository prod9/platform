package builder

import "dagger.io/dagger"

func withPNPMBuildBase(base *dagger.Container) *dagger.Container {
	return base.
		WithExec([]string{
			"apk", "add", "--no-cache",
			"nodejs-current", "build-base", "python3",
		}).
		WithExec([]string{"corepack", "enable", "pnpm"})
}

func withPNPMRunnerBase(base *dagger.Container) *dagger.Container {
	return base.
		WithExec([]string{
			"apk", "add", "--no-cache",
			"nodejs-current", "tzdata", "ca-certificates",
		}).
		WithExec([]string{"corepack", "enable", "pnpm"})
}

func withTypeModulePackageJSON(base *dagger.Container) *dagger.Container {
	return base.
		WithNewFile("/app/package.json", dagger.ContainerWithNewFileOpts{
			Contents: `{"type":"module"}`,
		})
}

func withPNPMPkgCache(sess *Session, base *dagger.Container) *dagger.Container {
	cache := sess.Client().CacheVolume("pnpm-store-cache")
	return base.WithMountedCache("/root/.local/share/pnpm", cache)
}
