package cmd

import (
	"fx.prodigy9.co/fxlog"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/srv"
)

var ServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Starts the platform server (API + web UI)",
	Run:   runServeCmd,
}

func runServeCmd(cmd *cobra.Command, args []string) {
	if err := srv.Serve(); err != nil {
		fxlog.Fatal(err)
	}
}
