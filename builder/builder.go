package builder

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/config"
)

var (
	ErrBadBuilder = errors.New("builder: invalid builder")
	ErrNoJobs     = errors.New("builder: empty jobs list, nothing to do.")
)

type BuildFunc func(
	ctx context.Context,
	client *dagger.Client,
	job *Job,
) (*dagger.Container, error)

type Builder struct {
	Name  string
	Build BuildFunc
}

var knownBuilders = map[string]Builder{
	"go/basic":     GoBasic,
	"go/workspace": GoWorkspace,
}

func FindBuilder(name string) (Builder, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	if builder, ok := knownBuilders[name]; ok {
		return builder, nil
	} else {
		return Builder{}, ErrBadBuilder
	}
}

func Build(cfg *config.Config, jobs ...*Job) error {
	if len(jobs) == 0 {
		return ErrNoJobs
	}

	ctx := context.Background()
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stderr))
	if err != nil {
		return err
	}
	defer client.Close()

	return errutil.AggregateWithTags(jobs, func(idx int, job *Job) (string, error) {
		ctx, cancel := context.WithTimeout(ctx, job.Timeout)
		defer cancel()

		container, err := job.Builder.Build(ctx, client, job)
		if err != nil {
			return job.Name, err
		}

		if job.Publish {
			log.Println("publishing", job.PublishImageName)
			hash, err := container.Publish(ctx, job.PublishImageName)
			if err != nil {
				return job.Name, err
			}
			log.Println("published", hash)
		}

		return job.Name, nil
	})
}
