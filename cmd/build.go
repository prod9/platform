package cmd

import (
	"context"
	"os"

	fxconfig "fx.prodigy9.co/config"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/engine"
	"platform.prodigy9.co/internal/buildlog"
)

var BuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Builds current directory",
	Run:   runBuild,
}

func runBuild(cmd *cobra.Command, args []string) {
	cfg, err := conf.Load(".")
	if err != nil {
		buildlog.Fatalln(err)
	}

	ctx := fxconfig.NewContext(context.Background(), fxconfig.Configure())
	sess := engine.NewSession(ctx)
	defer sess.Close()

	results, err := sess.Build(ctx, cfg, args, newObserver())
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
