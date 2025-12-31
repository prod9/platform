package builder

import "dagger.io/dagger"

const (
	// SEE: https://edu.chainguard.dev/open-source/wolfi/overview/
	BaseImageName = "cgr.dev/chainguard/wolfi-base"

	// CacheBuster can be updated when we need to ensure that both Dagger and Docker
	// are not caching bad old images in all environments.
	//
	// This should rarely need to be updated, but sometimes is necessary. For example,
	// chainguard might have a bad image uploaded to Docker Hub, and we need to force
	// a rebuild of the image in all environments.
	CacheBuster = "cache-buster-1aef8838"
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
