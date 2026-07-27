package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"dagger.io/dagger"
	fxconfig "fx.prodigy9.co/config"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/engine"
	"platform.prodigy9.co/framework"
)

var ListCmd = &cobra.Command{
	Use:   "ls",
	Short: "List files going into the container, for debugging purposes",
	RunE:  runList,
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := conf.Load(".")
	if err != nil {
		return err
	}

	// ls is a local debugging view, not a build: it wants the unit before anything runs, so
	// it assembles one itself at the local arch rather than through an engine entrypoint.
	units, err := framework.Units(cfg, args, cfg.LocalArch)
	if err != nil {
		return err
	}

	if len(units) == 0 {
		return errors.New("no modules to preview")
	}

	preview := units[0] // at least 1 by this point
	ctx := fxconfig.NewContext(context.Background(), fxconfig.Configure())
	sess := engine.NewSession(ctx)
	defer sess.Close()

	client, err := sess.Unsafe()
	if err != nil {
		return err
	}

	moddir := client.Host().
		Directory(preview.WorkDir, dagger.HostDirectoryOpts{
			Exclude: preview.Excludes,
		})

	stdout, err := framework.BaseImageForUnit(client, preview).
		WithExec([]string{"apk", "add", "--no-cache", "tree"}).
		WithDirectory(framework.SrcDir, moddir).
		WithExec([]string{"tree", "-L", "2", framework.SrcDir}).
		Stdout(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, stdout)
	return nil
}
