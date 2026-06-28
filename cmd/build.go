package cmd

import (
	"context"
	"os"

	fxconfig "fx.prodigy9.co/config"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/builder"
	"platform.prodigy9.co/engine"
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

	attempt, err := builder.AttemptFrom(cfg, args, builder.LocalBuild)
	if err != nil {
		buildlog.Fatalln(err)
	}

	eng := engine.New(fxconfig.Configure())
	defer eng.Close()

	ctx := engine.NewContext(context.Background(), eng)
	results, err := engine.Build(ctx, attempt)
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
