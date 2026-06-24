package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"platform.prodigy9.co/builder"
	"platform.prodigy9.co/internal/buildlog"
	"platform.prodigy9.co/project"
)

var BuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Builds current directory",
	Run:   runBuild,
}

func runBuild(cmd *cobra.Command, args []string) {
	cfg, err := project.Configure(".")
	if err != nil {
		buildlog.Fatalln(err)
	}

	jobs, err := builder.JobsFromArgs(cfg, args)
	if err != nil {
		buildlog.Fatalln(err)
	}

	sess, err := builder.NewSession(context.Background())
	if err != nil {
		buildlog.Fatalln(err)
	}
	defer sess.Close()

	results, err := builder.Build(sess, jobs...)
	if err != nil {
		buildlog.Fatalln(err)
	}

	anyerr := false
	for _, result := range results {
		if result.Err != nil {
			buildlog.Error(result.Err)
			anyerr = true
		}
	}
	if anyerr {
		os.Exit(1)
	}
}
