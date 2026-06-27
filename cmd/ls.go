package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"dagger.io/dagger"
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
	sess, err := engine.New(context.Background())
	if err != nil {
		buildlog.Fatalln(err)
	}
	defer sess.Close()

	moddir := sess.Client().Host().
		Directory(preview.WorkDir, dagger.HostDirectoryOpts{
			Exclude: preview.Excludes,
		})

	stdout, err := builder.BaseImageForUnit(sess, preview).
		WithExec([]string{"apk", "add", "--no-cache", "tree"}).
		WithDirectory("/app", moddir).
		WithExec([]string{"tree", "-L", "2"}).
		Stdout(sess.Context())
	if err != nil {
		buildlog.Fatalln(err)
	}

	fmt.Fprintln(os.Stdout, stdout)
}
