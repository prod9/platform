package framework

import "dagger.io/dagger"

// withGoVersion pins the build to the exact Go toolchain named in go.mod/go.work via Go's
// native GOTOOLCHAIN mechanism (Go 1.21+). The explicit `go version` probe materializes the
// toolchain into the module cache; setting GOTOOLCHAIN as a container env makes every later
// `go` invocation run that exact toolchain. Mount before withGoVersion so the download lands
// in the mod cache, and call before copying go.mod so this layer is keyed on the version
// alone — a go.mod change that keeps the version reuses the cached toolchain.
func withGoVersion(base *dagger.Container, goversion string) *dagger.Container {
	return base.
		WithEnvVariable("GOTOOLCHAIN", "go"+goversion).
		WithExec([]string{"go", "version"})
}

// withGoCaches mounts the persistent Go caches: modules (which also holds natively
// downloaded toolchains under golang.org/toolchain) and the compile/test-result cache.
func withGoCaches(client *dagger.Client, base *dagger.Container, goversion string) *dagger.Container {
	modcache := client.CacheVolume("platform-go-" + goversion + "-modcache")
	buildcache := client.CacheVolume("platform-go-" + goversion + "-buildcache")
	return base.
		WithMountedCache("/root/go/pkg/mod", modcache).
		WithMountedCache("/root/.cache/go-build", buildcache)
}
