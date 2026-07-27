package cmd

import (
	"context"

	fxconfig "fx.prodigy9.co/config"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/engine"
	"platform.prodigy9.co/internal/buildlog"
)

var CleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Prune the local Dagger build cache (clean-build reset)",
	RunE:  runCleanCmd,
}

func runCleanCmd(cmd *cobra.Command, args []string) error {
	ctx := fxconfig.NewContext(context.Background(), fxconfig.Configure())
	sess := engine.NewSession(ctx)
	defer sess.Close()

	if err := sess.Clean(ctx); err != nil {
		return err
	}

	buildlog.Event("dagger-cache", "cleaned")
	return nil
}
