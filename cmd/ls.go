package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"dagger.io/dagger"
	fxconfig "fx.prodigy9.co/config"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/builder"
	"platform.prodigy9.co/engine"
	"platform.prodigy9.co/internal/buildlog"
	"platform.prodigy9.co/project"
)

var ListCmd = &cobra.Command{
	Use:   "ls",
	Short: "List files going into the container, for debugging purposes",
	Run:   runList,
}

func runList(cmd *cobra.Command, args []string) {
	cfg, err := project.Configure(".")
	if err != nil {
		buildlog.Fatalln(err)
	}

	attempt, err := builder.AttemptFrom(cfg, args, builder.LocalBuild)
	if err != nil {
		buildlog.Fatalln(err)
	}

	if len(attempt.Units) == 0 {
		buildlog.Fatalln(errors.New("no modules to preview"))
	}

	preview := attempt.Units[0] // at least 1 by this point
	eng := engine.New(fxconfig.Configure())
	defer eng.Close()

	ctx := context.Background()
	client, err := eng.Client(ctx)
	if err != nil {
		buildlog.Fatalln(err)
	}

	moddir := client.Host().
		Directory(preview.WorkDir, dagger.HostDirectoryOpts{
			Exclude: preview.Excludes,
		})

	stdout, err := builder.BaseImageForUnit(client, preview).
		WithExec([]string{"apk", "add", "--no-cache", "tree"}).
		WithDirectory(builder.SrcDir, moddir).
		WithExec([]string{"tree", "-L", "2", builder.SrcDir}).
		Stdout(ctx)
	if err != nil {
		buildlog.Fatalln(err)
	}

	fmt.Fprintln(os.Stdout, stdout)
}
