package builder

import (
	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"strings"
)

type PNPMStatic struct{}

func (PNPMStatic) Name() string   { return "pnpm/static" }
func (PNPMStatic) Layout() Layout { return LayoutBasic }
func (PNPMStatic) Class() Class   { return ClassInterpreted }

func (b PNPMStatic) Discover(wd string) (map[string]Interface, error) {
	// since we'll need to delve into framework-specific stuff to find out,
	// which means we'll need to keep updated with the latest versions of every framework,
	// we're leaving this out for now
	return nil, ErrNoBuilder
}

func (b PNPMStatic) Build(sess *Session, job *Job) (container *dagger.Container, err error) {
	defer errutil.Wrap("pnpm/static", &err)

	host := sess.Client().Host().
		Directory(job.WorkDir, dagger.HostDirectoryOpts{Exclude: job.Excludes})

	builder := BaseImageForJob(sess, job)
	builder = withPNPMBuildBase(builder)
	builder = withPNPMPkgCache(sess, builder)

	builder = builder.
		WithFile("package.json", host.File("package.json")).
		WithFile("pnpm-lock.yaml", host.File("pnpm-lock.yaml")).
		WithExec([]string{"pnpm", "i"}).
		WithDirectory("/app", host).
		WithExec([]string{"pnpm", "build"})

	outdir := strings.TrimSpace(job.BuildDir)
	if outdir == "" {
		outdir = "build"
	}

	cmd := strings.TrimSpace(job.CommandName)
	if cmd == "" {
		cmd = "/usr/sbin/caddy"
	}

	args := []string{cmd}
	if len(job.CommandArgs) > 0 {
		args = append(args, job.CommandArgs...)
	} else {
		args = append(args, "run")
	}

	runner := BaseImageForJob(sess, job).
		WithExec([]string{"apk", "add", "--no-cache", "caddy", "tzdata", "ca-certificates"}).
		WithNewFile("/app/Caddyfile", dagger.ContainerWithNewFileOpts{Contents: `
			http://:3000 {
				root * /app/www
				file_server
			}
			`,
		}).
		WithDirectory("/app/www", builder.Directory(outdir)).
		WithDefaultArgs(dagger.ContainerWithDefaultArgsOpts{Args: args})

	return runner, nil
}
