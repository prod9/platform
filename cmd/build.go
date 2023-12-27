package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"platform.prodigy9.co/builder"
	"platform.prodigy9.co/internal/plog"
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
		plog.Fatalln(err)
	}

	jobs, err := builder.JobsFromArgs(cfg, args)
	if err != nil {
		plog.Fatalln(err)
	}

	sess, err := builder.NewSession(context.Background())
	if err != nil {
		plog.Fatalln(err)
	}
	defer sess.Close()

	results, err := builder.Build(sess, jobs...)
	if err != nil {
		plog.Fatalln(err)
	}

	for _, result := range results {
		if result.Err != nil {
			plog.Error(result.Err)
		}
	}
}
