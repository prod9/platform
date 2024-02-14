package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"dagger.io/dagger"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/builder"
	"platform.prodigy9.co/internal/plog"
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
		plog.Fatalln(err)
	}

	jobs, err := builder.JobsFromArgs(cfg, args)
	if err != nil {
		plog.Fatalln(err)
	}

	if len(jobs) == 0 {
		plog.Fatalln(errors.New("no modules to preview"))
	}

	preview := jobs[0] // at least 1 by this point
	sess, err := builder.NewSession(context.Background())
	if err != nil {
		plog.Fatalln(err)
	}
	defer sess.Close()

	moddir := sess.Client().Host().
		Directory(preview.WorkDir, dagger.HostDirectoryOpts{
			Exclude: preview.Excludes,
		})

	stdout, err := builder.BaseImageForJob(sess, preview).
		WithExec([]string{"apk", "add", "--no-cache", "tree"}).
		WithDirectory("/app", moddir).
		WithExec([]string{"tree", "-L", "2"}).
		Stdout(sess.Context())
	if err != nil {
		plog.Fatalln(err)
	}

	fmt.Fprintln(os.Stdout, stdout)
}
