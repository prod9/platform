package main

import (
	fxcmd "fx.prodigy9.co/cmd"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/cmd"
	"platform.prodigy9.co/internal/plog"
)

var rootCmd = &cobra.Command{
	Use:   "platform",
	Short: "PRODIGY9 platform swiss army knife",
}

var quiet int

func init() {
	rootCmd.PersistentFlags().CountVarP(&quiet, "quiet", "q", "less verbose logging")

	rootCmd.AddCommand(
		cmd.BootstrapCmd,
		cmd.BuildCmd,
		cmd.ConfigureCmd,
		cmd.DeployCmd,
		cmd.DiscoverCmd,
		cmd.ExportCmd,
		cmd.ListCmd,
		cmd.PreviewCmd,
		cmd.PublishCmd,
		cmd.ReleaseCmd,
		cmd.VanityCmd,

		fxcmd.PrintConfigCmd,
	)
}

func main() {
	defer plog.Event("exited")
	plog.SetQuietness(quiet)
	if err := rootCmd.Execute(); err != nil {
		plog.Fatalln(err)
	}
}
