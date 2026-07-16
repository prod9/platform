package clean

import (
	"context"

	fxconfig "fx.prodigy9.co/config"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/engine"
	"platform.prodigy9.co/internal/buildlog"
)

var Cmd = &cobra.Command{
	Use:   "clean",
	Short: "Prune the local Dagger build cache (clean-build reset)",
	Run:   run,
}

func run(cmd *cobra.Command, args []string) {
	eng := engine.New(fxconfig.Configure())
	defer eng.Close()

	buildlog.Event("pruning dagger build cache")
	if err := eng.Clean(context.Background()); err != nil {
		buildlog.Fatalln(err)
	}
	buildlog.Event("cache pruned")
}
