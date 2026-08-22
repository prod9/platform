package cmd

import (
	"context"

	fxconfig "fx.prodigy9.co/config"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/engine"
)

var BuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Builds current directory",
	RunE:  runBuild,
}

func runBuild(cmd *cobra.Command, args []string) error {
	cfg, err := conf.Load(".")
	if err != nil {
		return err
	}

	ctx := fxconfig.NewContext(context.Background(), fxconfig.Configure())
	sess := engine.NewSession(ctx)
	defer sess.Close()

	results, err := sess.Build(ctx, cfg, args, newObserver())
	if err != nil {
		return err
	}

	return failedUnits(results)
}
