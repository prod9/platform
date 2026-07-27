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
)

var ExecCmd = &cobra.Command{
	Use:   "exec [module] [-- command...]",
	Short: "Run a command in, or open a shell into, the built container",
	RunE:  runExec,
}

func runExec(cmd *cobra.Command, args []string) error {
	selectors, command := splitAtDash(cmd, args)

	cfg, err := conf.Load(".")
	if err != nil {
		return err
	}

	modname, err := selectModule(cfg, selectors)
	if err != nil {
		return err
	}

	ctx := fxconfig.NewContext(context.Background(), fxconfig.Configure())
	sess := engine.NewSession(ctx)
	defer sess.Close()

	results, err := sess.Build(ctx, cfg, []string{modname}, newObserver())
	if err != nil {
		return err
	}

	result := results[0]
	if result.Err != nil {
		return result.Err
	}

	// A given command runs non-interactively (scriptable, smoke-friendly); a bare invocation
	// opens a shell for a human, or prints an inspectable summary when stdout isn't a terminal.
	container := result.UnsafeContainer()
	switch {
	case len(command) > 0:
		return runInContainer(ctx, container, command)
	case isTerminal(os.Stdout):
		return openShell(ctx, container)
	default:
		return printSummary(ctx, container)
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

// runInContainer runs the operator's command and makes its status this invocation's status,
// which is what makes `platform exec -- <cmd>` scriptable. The code travels back as an
// exitError rather than an os.Exit here, so the session that owns this container closes
// first.
func runInContainer(ctx context.Context, container *dagger.Container, command []string) error {
	exec := container.WithExec(command, dagger.ContainerWithExecOpts{
		Expect: dagger.ReturnTypeAny,
	})

	stdout, err := exec.Stdout(ctx)
	if err != nil {
		return err
	}
	stderr, err := exec.Stderr(ctx)
	if err != nil {
		return err
	}
	code, err := exec.ExitCode(ctx)
	if err != nil {
		return err
	}

	fmt.Fprint(os.Stdout, stdout)
	fmt.Fprint(os.Stderr, stderr)
	if code != 0 {
		return exitError{code: code}
	}
	return nil
}

func openShell(ctx context.Context, container *dagger.Container) error {
	_, err := container.Terminal().Sync(ctx)
	return err
}

func printSummary(ctx context.Context, container *dagger.Container) error {
	command, err := container.DefaultArgs(ctx)
	if err != nil {
		return err
	}
	workdir, err := container.Workdir(ctx)
	if err != nil {
		return err
	}
	envs, err := container.EnvVariables(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, "workdir:", workdir)
	fmt.Fprintln(os.Stdout, "command:", strings.Join(command, " "))
	fmt.Fprintln(os.Stdout, "env:")
	for _, env := range envs {
		name, err := env.Name(ctx)
		if err != nil {
			return err
		}
		value, err := env.Value(ctx)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "  %s=%s\n", name, value)
	}
	return nil
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
