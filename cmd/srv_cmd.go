package cmd

import (
	"fx.prodigy9.co/fxlog"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/srv"
)

var SrvCmd = &cobra.Command{
	Use:     "srv",
	Aliases: []string{"serve"},
	Short:   "Starts the platform server (API + web UI)",
	Run:     runSrvCmd,
}

func runSrvCmd(cmd *cobra.Command, args []string) {
	if err := srv.Serve(); err != nil {
		fxlog.Fatal(err)
	}
}
