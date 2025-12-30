package builder

import (
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/builder/fileutil"
	"platform.prodigy9.co/builder/gowork"
)

type GoBasic struct{}

func (GoBasic) Name() string   { return "go/basic" }
func (GoBasic) Layout() Layout { return LayoutBasic }
func (GoBasic) Class() Class   { return ClassNative }

func (b GoBasic) Discover(wd string) (map[string]Interface, error) {
	if detected, err := fileutil.DetectFile(wd, "go.mod"); err != nil {
		return nil, err
	} else if !detected {
		return nil, ErrNoBuilder
	}

	name := filepath.Base(wd)
	return map[string]Interface{name: b}, nil
}

func (GoBasic) Build(sess *Session, job *Job) (container *dagger.Container, err error) {
	defer errutil.Wrap("go/basic", &err)

	host := sess.Client().Host().Directory(job.WorkDir, dagger.HostDirectoryOpts{
		Exclude: job.Excludes,
	})

	goversion, _, err := gowork.ParseFile(filepath.Join(job.WorkDir, "go.mod"))
	if err != nil {
		return nil, err
	}

	cmd := strings.TrimSpace(job.CommandName)
	switch {
	case cmd == "" && job.PackageName != "":
		cmd = job.PackageName
	case cmd == "" && job.Name != "":
		cmd = job.Name
	}

	args := []string{"./" + cmd}
	if len(job.CommandArgs) > 0 {
		args = append(args, job.CommandArgs...)
	}

	base := BaseImageForJob(sess, job)

	builder := withGoBuildBase(base)
	builder = withGoPkgCache(sess, builder, goversion)
	builder, gobin := withGoVersion(builder, goversion)

	builder = builder.
		WithFile("go.mod", host.File("go.mod")).
		WithFile("go.sum", host.File("go.sum")).
		WithExec([]string{gobin, "mod", "download", "-x", "all"})

	builder = builder.
		WithDirectory(".", host).
		WithExec([]string{gobin, "test", "-v", "./..."}).
		WithExec([]string{gobin, "build", "-v", "-o", "/out/" + cmd, job.PackageName})

	runner := withGoRunnerBase(base)
	runner = withJobEnv(runner, job)

	runner = runner.WithFile("/app/"+cmd, builder.File("/out/"+cmd))
	for _, dir := range job.AssetDirs {
		runner = runner.WithDirectory(dir, builder.Directory(dir))
	}

	runner = runner.WithDefaultArgs(args)
	return runner.Sync(sess.Context())
}
