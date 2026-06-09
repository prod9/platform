// Package builder produces container images for projects discovered by platform.
//
// # Base image policy
//
// Every builder in this package starts from Chainguard's Wolfi base image
// (cgr.dev/chainguard/wolfi-base) via [BaseImageForJob]. This is the standard
// and gives us a small, regularly-patched, glibc-free base shared across all
// language stacks (Go native, Go workspace, pnpm basic/static/workspace).
//
// The sole exception is the [Dockerfile] builder, which by definition uses the
// user-supplied Dockerfile's FROM line. That builder is intentionally
// discouraged: it bypasses Wolfi, the apk cache mount, and our package
// conventions in [withBuildPkgs] / [withRunnerPkgs]. It emits a runtime warning
// when invoked. Prefer one of the language-specific builders whenever possible.
package builder

import "dagger.io/dagger"

const (
	// SEE: https://edu.chainguard.dev/open-source/wolfi/overview/
	//
	// Pinned by digest (the multi-arch index digest — Dagger picks the right
	// per-platform manifest at build time). Chainguard's :latest is a floating
	// ref, so reproducibility wins over readability here. Refresh manually on a
	// monthly cadence to absorb base-layer CVEs; userland is already refreshed
	// every build via `apk update && apk upgrade` in [BaseImageForJob].
	//
	// To refresh:
	//   docker buildx imagetools inspect cgr.dev/chainguard/wolfi-base:latest
	// then update both BaseImageName and CacheBuster (keep them in sync — the
	// cache buster's hex is the first 8 chars of the digest below).
	BaseImageName = "cgr.dev/chainguard/wolfi-base@sha256:b78bb982194828b6c9c214230bf34d51944e2102ea8468f01ac21e5f99328efd"

	// CacheBuster forces Dagger and Docker to invalidate cached base layers
	// across all environments. Bumped in lockstep with [BaseImageName] above so
	// a base-image refresh always re-pulls; can also be bumped on its own if
	// Chainguard ships a bad image at the same digest (rare).
	CacheBuster = "cache-buster-b78bb982"
)

func BaseImageForJob(sess *Session, job *Job) *dagger.Container {
	apkCache := sess.Client().CacheVolume("platform-apk-cache")

	return sess.Client().
		Container(dagger.ContainerOpts{
			Platform: dagger.Platform(job.Platform),
		}).
		From(BaseImageName).
		WithWorkdir("/app").
		WithLabel("org.opencontainers.image.source", job.Repository).
		WithExec([]string{"mkdir", "-p", "/app", "/out"}).
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

func withJobEnv(base *dagger.Container, job *Job) *dagger.Container {
	for key, value := range job.Env {
		base = base.WithEnvVariable(key, value)
	}
	return base
}
