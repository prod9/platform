package cmd

import (
	"errors"

	"fx.prodigy9.co/app"
	fxcmd "fx.prodigy9.co/cmd"
	"fx.prodigy9.co/config"
	"fx.prodigy9.co/secret"
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
	cmd.PreRunE = func(*cobra.Command, []string) error {
		if _, ok := config.GetOK(config.Configure(), secret.SecretConfig); !ok {
			return errors.New("srv: SECRET must be set before startup (configure it in the environment)")
		}
		return nil
	}

	cmd.AddCommand(app.CollectCommands(srv.App)...)
	return cmd
}
