package builder

import "dagger.io/dagger"

const BaseImageName = "rockylinux:9-minimal"

func BaseImageForJob(sess *Session, job *Job) *dagger.Container {
	return sess.Client().
		Container(dagger.ContainerOpts{
			Platform: dagger.Platform(job.Platform),
		}).
		From(BaseImageName).
		WithWorkdir("/app").
		WithLabel("org.opencontainers.image.source", job.Repository).
		WithExec([]string{"mkdir", "-p", "/app", "/out"})
}
