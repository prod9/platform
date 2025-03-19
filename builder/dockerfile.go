package builder

import (
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/builder/fileutil"
)

type Dockerfile struct{}

var _ Interface = Dockerfile{}

func (d Dockerfile) Name() string   { return "dockerfile" }
func (d Dockerfile) Layout() Layout { return LayoutBasic }
func (d Dockerfile) Class() Class   { return ClassCustom }

func (d Dockerfile) Discover(wd string) (map[string]Interface, error) {
	if detected, err := fileutil.DetectFile(wd, "Dockerfile"); err != nil {
		return nil, err
	} else if !detected {
		return nil, ErrNoBuilder
	}

	name := filepath.Base(wd)
	return map[string]Interface{name: d}, nil
}

func (d Dockerfile) Build(sess *Session, job *Job) (container *dagger.Container, err error) {
	defer errutil.Wrap("dockerfile", &err)

	host := sess.Client().Host().Directory(job.WorkDir, dagger.HostDirectoryOpts{
		Exclude: job.Excludes,
	})

	cmd := strings.TrimSpace(job.CommandName)
	var args []string
	if cmd != "" {
		args = append(args, cmd)
	}
	if len(job.CommandArgs) > 0 {
		args = append(args, job.CommandArgs...)
	}

	// not using BaseImageForJob because, well, dockerfiles have their own bases
	// this builder should be discouraged
	opts := dagger.DirectoryDockerBuildOpts{}
	if len(job.Env) > 0 {
		for key, value := range job.Env {
			opts.BuildArgs = append(opts.BuildArgs,
				dagger.BuildArg{Name: key, Value: value},
			)
		}
	}

	builder := host.DockerBuild(dagger.DirectoryDockerBuildOpts{
		Platform:   dagger.Platform(job.Platform),
		Dockerfile: "",
		Target:     "",
		BuildArgs:  []dagger.BuildArg{},
		Secrets:    []*dagger.Secret{},
	})

	builder = withJobEnv(builder, job)
	if len(args) > 0 {
		builder = builder.WithDefaultArgs(args)
	}

	return builder.Sync(sess.Context())
}
