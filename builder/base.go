package builder

import "dagger.io/dagger"

const BASE_IMAGE = "alpine:3.18"

func BaseImageForJob(client *dagger.Client, job *Job) *dagger.Container {
	return client.
		Container(dagger.ContainerOpts{Platform: dagger.Platform(job.Platform)}).
		From(BASE_IMAGE).
		WithWorkdir("/app").
		WithLabel("org.opencontainers.image.source", job.Repository)
}
