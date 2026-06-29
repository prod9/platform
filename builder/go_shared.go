package builder

import (
	"strings"

	"dagger.io/dagger"
)

// goAppBin resolves the name of the application binary a Go build emits: the
// operator's command name, else the Go package name, else the module's folder name
// (a module in api/ builds a binary named "api"). Distinct from gobin (the Go
// toolchain binary, go1.x) and from the package passed to `go build` as the target.
func goAppBin(unit *BuildUnit) string {
	name := strings.TrimSpace(unit.CommandName)
	switch {
	case name == "" && unit.PackageName != "":
		name = unit.PackageName
	case name == "" && unit.Name != "":
		name = unit.Name
	}
	return name
}

func withGoVersion(base *dagger.Container, goversion string) (*dagger.Container, string) {
	gobin := "/root/go/bin/go" + goversion
	base = base.
		WithExec([]string{"/usr/bin/go", "install", "golang.org/dl/go" + goversion + "@latest"}).
		WithExec([]string{gobin, "download"})

	return base, gobin
}

func withGoPkgCache(client *dagger.Client, base *dagger.Container, goversion string) *dagger.Container {
	modcache := client.CacheVolume("platform-go-" + goversion + "-modcache")
	return base.WithMountedCache("/root/go/pkg/mod", modcache)
}
