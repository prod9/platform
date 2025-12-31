package builder

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/builder/fileutil"
	"platform.prodigy9.co/builder/gowork"
)

type GoWorkspace struct{}

func (GoWorkspace) Name() string   { return "go/workspace" }
func (GoWorkspace) Layout() Layout { return LayoutWorkspace }
func (GoWorkspace) Class() Class   { return ClassNative }

func (b GoWorkspace) Discover(wd string) (map[string]Interface, error) {
	if detected, err := fileutil.DetectFile(wd, "go.work"); err != nil {
		return nil, err
	} else if !detected {
		return nil, ErrNoBuilder
	}

	// scan for go/basic on subfolders, should switch to proper go.work parsers if/when it
	// is available from go tooling directly
	mods := map[string]Interface{}
	err := fileutil.WalkSubdirs(wd, func(dir os.DirEntry) error {
		submods, err := GoBasic{}.Discover(filepath.Join(wd, dir.Name()))
		if errors.Is(err, ErrNoBuilder) {
			return nil
		}

		// found a go/basic submodule, mark it as using go/workspace
		for submod := range submods {
			mods[submod] = b
		}
		return nil
	})

	if err != nil {
		return nil, err
	} else if len(mods) == 0 {
		return nil, ErrNoBuilder
	} else {
		return mods, nil
	}
}

func (GoWorkspace) Build(sess *Session, job *Job) (container *dagger.Container, err error) {
	defer errutil.Wrap("go/workspace", &err)

	wsdir, err := filepath.Abs(filepath.Join(job.WorkDir, ".."))
	if err != nil {
		return nil, err
	}

	host := sess.Client().Host().Directory(wsdir, dagger.HostDirectoryOpts{
		Exclude: job.Excludes,
	})

	workfile := filepath.Join(wsdir, "go.work")
	goversion, workmods, err := gowork.ParseFile(workfile)
	if err != nil {
		return nil, err
	}

	// prepare job parameters
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

	// build
	base := BaseImageForJob(sess, job)

	builder := withBuildPkgs(base, "go")
	builder, gobin := withGoVersion(builder, goversion)
	builder = withGoPkgCache(sess, builder, goversion)

	builder = builder.
		WithFile("go.work", host.File("go.work")).
		WithFile("go.work.sum", host.File("go.work.sum"))

	// needs all go.mod of all modules to start dependencies check
	// otherwise it'll try to fetch them from the internet during build
	for _, mod := range workmods {
		builder = builder.
			WithFile("/app/"+mod+"/go.mod", host.File("./"+mod+"/go.mod")).
			WithFile("/app/"+mod+"/go.sum", host.File("./"+mod+"/go.sum"))
	}

	// NOTE: Users should `go work sync` if mod doesn't match as build logs maybe invisible
	// or hard to track down for the user.
	builder = builder.
		WithExec([]string{gobin, "mod", "download", "-x", "all"})

	testargs := []string{gobin, "test", "-v"}
	for _, mod := range workmods {
		testargs = append(testargs, "./"+mod+"/...")
	}

	pkg := job.PackageName
	if pkg == "" {
		pkg = "./" + job.Name
	}

	builder = builder.
		WithDirectory(".", host).
		WithExec(testargs).
		WithExec([]string{gobin, "build", "-v", "-o", "/out/" + cmd, pkg})

	// run
	runner := withRunnerPkgs(base)
	runner = withJobEnv(runner, job)
	runner = runner.WithFile("/app/"+cmd, builder.File("/out/"+cmd))
	for _, dir := range job.AssetDirs {
		runner = runner.WithDirectory(dir, builder.Directory(dir))
	}

	runner = runner.WithDefaultArgs(args)
	return runner.Sync(sess.Context())
}
