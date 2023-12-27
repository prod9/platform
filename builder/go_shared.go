package builder

import "dagger.io/dagger"

func withGoBuildBase(base *dagger.Container) *dagger.Container {
	return base.
		WithExec([]string{
			"apk", "add", "--no-cache",
			"build-base", "git", "go", "musl",
			"ca-certificates", "curl",
		})
}

func withGoRunnerBase(base *dagger.Container) *dagger.Container {
	return base.
		WithExec([]string{"apk", "add", "--no-cache", "ca-certificates", "tzdata"})
}

func withGoMUSLPatch(base *dagger.Container) *dagger.Container {
	return base.
		WithExec([]string{"curl", "-sLo", "/etc/apk/keys/sgerrand.rsa.pub", "https://alpine-pkgs.sgerrand.com/sgerrand.rsa.pub"}).
		WithExec([]string{"curl", "-sLo", "glibc.apk", "https://github.com/sgerrand/alpine-pkg-glibc/releases/download/2.34-r0/glibc-2.34-r0.apk"}).
		WithExec([]string{"apk", "add", "--force-overwrite", "--no-cache", "glibc.apk"})
}

func withCustomGoVersion(base *dagger.Container, goversion string) (*dagger.Container, string) {
	gobin := "/root/go/bin/go" + goversion
	base = base.
		WithEnvVariable("GOROOT", "/usr/lib/go").
		WithExec([]string{"go", "install", "golang.org/dl/go" + goversion + "@latest"}).
		WithExec([]string{gobin, "download"})

	return base, gobin
}

func withGoPkgCache(sess *Session, base *dagger.Container, goversion string) *dagger.Container {
	modcache := sess.Client().CacheVolume("go-" + goversion + "-modcache")
	return base.WithMountedCache("/root/go/pkg/mod", modcache)
}
