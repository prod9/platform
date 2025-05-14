package builder

import "dagger.io/dagger"

const (
	BaseImageName = "rockylinux:9-minimal"

	// CacheBusterName can be updated when we need to ensure that both Dagger and Docker
	// are not caching bad old images in all environments.
	//
	// This should rarely need to be updated, but sometimes is necessary. For example,
	// rockylinux might have a bad image uploaded to Docker Hub, and we need to force
	// a rebuild of the image in all environments.
	CacheBusterName = "cache-buster-0d8d5fe5"

	DNFConf = `
[main]
gpgcheck=1
installonly_limit=3
clean_requirements_on_remove=True
best=False
skip_if_unavailable=False
fastestmirror=True
cacheonly=True
minrate=40k
max_parallel_downloads=8
timeout=5
`
)

func BaseImageForJob(sess *Session, job *Job) *dagger.Container {
	dnfCache := sess.Client().CacheVolume("platform-dnf-cache")

	return sess.Client().
		Container(dagger.ContainerOpts{
			Platform: dagger.Platform(job.Platform),
		}).
		From(BaseImageName).
		WithWorkdir("/app").
		WithLabel("org.opencontainers.image.source", job.Repository).
		WithExec([]string{"mkdir", "-p", "/app", "/out"}).
		WithNewFile("/"+CacheBusterName, CacheBusterName).

		// optimize dnf
		WithMountedCache("/var/cache/dnf", dnfCache).
		WithNewFile("/etc/dnf/dnf.conf", DNFConf).
		WithExec([]string{"microdnf", "install", "-y", "epel-release"}).
		WithExec([]string{"microdnf", "makecache", "-y"}).
		WithExec([]string{"microdnf", "update", "-y"})
}

func withCaddyServer(base *dagger.Container) *dagger.Container {
	return base.
		WithExec([]string{"microdnf", "install", "-y", "caddy"})
}
