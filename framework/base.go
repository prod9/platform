// Package framework defines the sole owners of project types: a Framework recognizes
// its stack (Discover), scaffolds it (Scaffold), and builds its container image (Build).
// BuildUnit is the resolved work definition the engine executes; Units turns config into
// one per selected module.
//
// # Base image policy
//
// Every framework in this package starts from Chainguard's Wolfi base image
// (cgr.dev/chainguard/wolfi-base) via [BaseImageForUnit]. This is the standard
// and gives us a small, regularly-patched, glibc-free base shared across all
// language stacks (Go native, Go workspace, pnpm basic/static/workspace).
//
// The sole exception is the [Dockerfile] framework, which by definition uses the
// user-supplied Dockerfile's FROM line. That framework is intentionally
// discouraged: it bypasses Wolfi, the apk cache mount, and our package
// conventions in [withBuildPkgs] / [withRunnerPkgs]. It emits a runtime warning
// when invoked. Prefer one of the language-specific frameworks whenever possible.
package framework

import "dagger.io/dagger"

const (
	// SEE: https://edu.chainguard.dev/open-source/wolfi/overview/
	//
	// Never pinned — platform names no version, and Wolfi is rolling anyway: the
	// repository carries exactly one real tag. Userland is refreshed every build
	// via `apk update && apk upgrade` in [BaseImageForUnit].
	BaseImageName = "cgr.dev/chainguard/wolfi-base:latest"

	// CacheBuster forces Dagger and Docker to invalidate cached base layers
	// across all environments. Bump it to shed a stale base layer.
	CacheBuster = "cache-buster-1"
)

// The platform runtime filesystem convention — a small FHS-style tree every
// framework lays down, so an operator shelling into a built container always finds
// things in the same place: sources under src, executables on PATH under bin, and
// the app's working directory (assets, data) under run.
const (
	SrcDir = "/platform/src" // build workspace: host sources compile here
	BinDir = "/platform/bin" // compiled executables, on PATH
	RunDir = "/platform/run" // runtime working directory (assets, data)
)

func BaseImageForUnit(client *dagger.Client, unit *BuildUnit) *dagger.Container {
	apkCache := client.CacheVolume("platform-apk-cache")

	return client.
		Container(dagger.ContainerOpts{
			Platform: dagger.Platform(unit.Arch),
		}).
		From(BaseImageName).
		WithLabel("org.opencontainers.image.source", unit.RepositoryURL()).
		WithExec([]string{"mkdir", "-p", SrcDir, BinDir, RunDir}).
		WithEnvVariable("PATH", BinDir+":${PATH}", dagger.ContainerWithEnvVariableOpts{Expand: true}).
		WithWorkdir(RunDir).
		WithNewFile("/"+CacheBuster, CacheBuster).

		// optimize dnf
		WithMountedCache("/var/cache/apk", apkCache).
		WithExec([]string{"apk", "update"}).
		WithExec([]string{"apk", "upgrade"})
}

func withPkgs(base *dagger.Container, pkgs ...string) *dagger.Container {
	return base.WithExec(append([]string{"apk", "add"}, pkgs...))
}

func withBuildPkgs(base *dagger.Container, pkgs ...string) *dagger.Container {
	set := []string{"build-base", "git", "curl", "bash"}
	return withPkgs(base, append(set, pkgs...)...)
}

func withRunnerPkgs(base *dagger.Container, pkgs ...string) *dagger.Container {
	set := []string{"ca-certificates", "curl", "netcat-openbsd", "tzdata"}
	return withPkgs(base, append(set, pkgs...)...)
}

func withCaddyServer(base *dagger.Container) *dagger.Container {
	return withPkgs(base, "caddy")
}

func withUnitEnv(base *dagger.Container, unit *BuildUnit) *dagger.Container {
	for key, value := range unit.Env {
		base = base.WithEnvVariable(key, value)
	}
	return base
}

func withUnitAssets(runner, builder *dagger.Container, unit *BuildUnit) *dagger.Container {
	for _, dir := range unit.AssetDirs {
		runner = runner.WithDirectory(dir, builder.Directory(dir))
	}
	return runner
}
