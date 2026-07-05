package builder

import "dagger.io/dagger"

func withGoVersion(base *dagger.Container, goversion string) (*dagger.Container, string) {
	gobin := "/root/go/bin/go" + goversion
	base = base.
		WithExec([]string{"/usr/bin/go", "install", "golang.org/dl/go" + goversion + "@latest"}).
		WithExec([]string{gobin, "download"})

	return base, gobin
}

// withGoCaches mounts every persistent Go cache: modules, the compile/test-result cache,
// and the downloaded SDK. Mount before withGoVersion so the toolchain download lands in the
// SDK volume and survives a dagger layer-cache bust, instead of re-pulling ~150MB each time.
func withGoCaches(client *dagger.Client, base *dagger.Container, goversion string) *dagger.Container {
	modcache := client.CacheVolume("platform-go-" + goversion + "-modcache")
	buildcache := client.CacheVolume("platform-go-" + goversion + "-buildcache")
	sdkcache := client.CacheVolume("platform-go-sdk")
	return base.
		WithMountedCache("/root/go/pkg/mod", modcache).
		WithMountedCache("/root/.cache/go-build", buildcache).
		WithMountedCache("/root/sdk", sdkcache)
}
