package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"dagger.io/dagger"
	fxconfig "fx.prodigy9.co/config"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/engine"
	"platform.prodigy9.co/internal/buildlog"
)

var ExecCmd = &cobra.Command{
	Use:   "exec [module] [-- command...]",
	Short: "Run a command in, or open a shell into, the built container",
	Run:   runExec,
}

func runExec(cmd *cobra.Command, args []string) {
	selectors, command := splitAtDash(cmd, args)

	cfg, err := conf.Load(".")
	if err != nil {
		buildlog.Fatalln(err)
	}

	modname, err := selectModule(cfg, selectors)
	if err != nil {
		buildlog.Fatalln(err)
	}

	ctx := fxconfig.NewContext(context.Background(), fxconfig.Configure())
	sess := engine.NewSession(ctx)
	defer sess.Close()

	results, err := sess.Build(ctx, cfg, []string{modname}, newObserver())
	if err != nil {
		buildlog.Fatalln(err)
	}

	result := results[0]
	if result.Err != nil {
		buildlog.Fatalln(result.Err)
	}

	// A given command runs non-interactively (scriptable, smoke-friendly); a bare invocation
	// opens a shell for a human, or prints an inspectable summary when stdout isn't a terminal.
	container := result.UnsafeContainer()
	switch {
	case len(command) > 0:
		runInContainer(ctx, container, command)
	case isTerminal(os.Stdout):
		openShell(ctx, container)
	default:
		printSummary(ctx, container)
	}
}

// splitAtDash separates module selectors (before --) from the command to run (after --).
func splitAtDash(cmd *cobra.Command, args []string) (selectors, command []string) {
	dash := cmd.ArgsLenAtDash()
	if dash < 0 {
		return args, nil
	}
	return args[:dash], args[dash:]
}

func runInContainer(ctx context.Context, container *dagger.Container, command []string) {
	exec := container.WithExec(command, dagger.ContainerWithExecOpts{
		Expect: dagger.ReturnTypeAny,
	})

	stdout, err := exec.Stdout(ctx)
	if err != nil {
		buildlog.Fatalln(err)
	}
	stderr, err := exec.Stderr(ctx)
	if err != nil {
		buildlog.Fatalln(err)
	}
	code, err := exec.ExitCode(ctx)
	if err != nil {
		buildlog.Fatalln(err)
	}

	fmt.Fprint(os.Stdout, stdout)
	fmt.Fprint(os.Stderr, stderr)
	os.Exit(code)
}

func openShell(ctx context.Context, container *dagger.Container) {
	if _, err := container.Terminal().Sync(ctx); err != nil {
		buildlog.Fatalln(err)
	}
}

func printSummary(ctx context.Context, container *dagger.Container) {
	command, err := container.DefaultArgs(ctx)
	if err != nil {
		buildlog.Fatalln(err)
	}
	workdir, err := container.Workdir(ctx)
	if err != nil {
		buildlog.Fatalln(err)
	}
	envs, err := container.EnvVariables(ctx)
	if err != nil {
		buildlog.Fatalln(err)
	}

	fmt.Fprintln(os.Stdout, "workdir:", workdir)
	fmt.Fprintln(os.Stdout, "command:", strings.Join(command, " "))
	fmt.Fprintln(os.Stdout, "env:")
	for _, env := range envs {
		name, err := env.Name(ctx)
		if err != nil {
			buildlog.Fatalln(err)
		}
		value, err := env.Value(ctx)
		if err != nil {
			buildlog.Fatalln(err)
		}
		fmt.Fprintf(os.Stdout, "  %s=%s\n", name, value)
	}
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
