// Package framework defines the sole owners of project types: a Framework recognizes
// its stack (Discover), scaffolds it (Scaffold), and knows how it is built (Plan/Execute).
// BuildUnit is the resolved work definition the engine executes; Units turns config into
// one per selected module.
//
// # Base image policy
//
// Every framework in this package starts from Chainguard's Wolfi base image
// (cgr.dev/chainguard/wolfi-base) via [BaseImageForUnit]. This is the standard
// and gives us a small, regularly-patched base shared across all language
// stacks (Go native, Go workspace, pnpm basic/static/workspace).
//
// Wolfi is glibc-based, not musl — stock Linux binaries and CGO builds run
// unmodified, and no package here ever wants a musl variant.
//
// The sole exception is the [Dockerfile] framework, which by definition uses the
// user-supplied Dockerfile's FROM line. That framework is intentionally
// discouraged: it bypasses Wolfi, the apk cache mount, and our package
// conventions in [withBuildPkgs] / [withRunnerPkgs]. It emits a runtime warning
// when invoked. Prefer one of the language-specific frameworks whenever possible.
package framework

import (
	"fmt"

	"dagger.io/dagger"
)

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

		// The apk cache persists across builds, so the refresh below re-downloads only
		// what actually moved in Wolfi since the last one.
		WithMountedCache("/var/cache/apk", apkCache).
		WithExec([]string{"apk", "update"}).
		WithExec([]string{"apk", "upgrade"})
}

// unitHost is the unit's own directory on the host, minus its excludes. Workspace layouts
// need the workspace root instead and call hostDir directly.
func unitHost(client *dagger.Client, unit *BuildUnit) *dagger.Directory {
	return hostDir(client, unit.WorkDir, unit)
}

func hostDir(client *dagger.Client, dir string, unit *BuildUnit) *dagger.Directory {
	return client.Host().Directory(dir, dagger.HostDirectoryOpts{Exclude: unit.Excludes})
}

// unknownStep guards the Execute switch. It is unreachable while Plan and Execute agree —
// which is exactly why it must stay loud: a step added to one and not the other is a
// silently skipped build stage otherwise.
func unknownStep(step Step) error {
	return fmt.Errorf("%w: %s", ErrUnknownStep, step)
}

// These names resolve against Wolfi, not Alpine — the two share apk and share most
// package names, so an Alpine lookup is wrong in exactly the cases that matter. Check a
// name against the image itself: docs/vendor/wolfi.md.
func withPkgs(base *dagger.Container, pkgs ...string) *dagger.Container {
	return base.WithExec(append([]string{"apk", "add"}, pkgs...))
}

func withBuildPkgs(base *dagger.Container, pkgs ...string) *dagger.Container {
	set := []string{"build-base", "git", "curl", "bash"}
	return withPkgs(base, append(set, pkgs...)...)
}

func withRunnerPkgs(base *dagger.Container, pkgs ...string) *dagger.Container {
	set := []string{"ca-certificates", "curl", "mailcap", "netcat-openbsd", "tzdata"}
	return withPkgs(base, append(set, pkgs...)...)
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
