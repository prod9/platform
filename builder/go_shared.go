package builder

import "dagger.io/dagger"

func withGoBuildBase(base *dagger.Container) *dagger.Container {
	return base.
		WithExec([]string{
			"microdnf", "install", "-y",
			"golang", "git", "which",
		})
}

func withGoRunnerBase(base *dagger.Container) *dagger.Container {
	return base.
		WithExec([]string{
			"microdnf", "install", "-y",
			"ca-certificates", "tzdata",
		})
}

func withCustomGoVersion(base *dagger.Container, goversion string) (*dagger.Container, string) {
	gobin := "/root/go/bin/go" + goversion
	base = base.
		WithExec([]string{"/usr/bin/go", "install", "golang.org/dl/go" + goversion + "@latest"}).
		WithExec([]string{gobin, "download"})

	return base, gobin
}

func withGoPkgCache(sess *Session, base *dagger.Container, goversion string) *dagger.Container {
	modcache := sess.Client().CacheVolume("go-" + goversion + "-modcache")
	return base.WithMountedCache("/root/go/pkg/mod", modcache)
}
