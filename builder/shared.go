package builder

import "dagger.io/dagger"

func withJobEnv(base *dagger.Container, job *Job) *dagger.Container {
	for key, value := range job.Env {
		base = base.WithEnvVariable(key, value)
	}
	return base
}
