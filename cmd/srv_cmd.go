package cmd

import (
	"fx.prodigy9.co/app"
	fxcmd "fx.prodigy9.co/cmd"
	"fx.prodigy9.co/config"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/srv"
)

var SrvCmd = buildSrvCmd()

func buildSrvCmd() *cobra.Command {
	fragment := app.CollectFragment(srv.App)
	cmd := fxcmd.BuildServeCommandFromFragments(fragment)
	cmd.Use = "srv"
	cmd.Aliases = []string{"serve"}
	cmd.Short = "Starts the platform server (API + web UI)"
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		return srv.ValidateBoot(cmd.Context(), config.Configure())
	}

	cmd.AddCommand(app.CollectCommands(srv.App)...)
	return cmd
}
