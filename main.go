package main

import (
	"os"

	fxcmd "fx.prodigy9.co/cmd"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/cmd"
	"platform.prodigy9.co/internal/plog"
	"platform.prodigy9.co/project"
)

var rootCmd = &cobra.Command{
	Use:   "platform",
	Short: "PRODIGY9 platform swiss army knife",
}

var (
	quietness int
	verbosity int
)

func init() {
	rootCmd.PersistentFlags().CountVarP(&quietness, "quiet", "q", "less verbose logging")
	rootCmd.PersistentFlags().CountVarP(&verbosity, "verbose", "v", "more verbose logging")

	rootCmd.PersistentFlags().StringVarP(&project.PlatformFilename, "file", "f",
		project.PlatformFilename, "specify a different platform.toml to load")

	rootCmd.AddCommand(
		cmd.BootstrapCmd,
		cmd.BuildCmd,
		cmd.ConfigureCmd,
		cmd.DeployCmd,
		cmd.DiscoverCmd,
		cmd.ExportCmd,
		cmd.GitCtxCmd,
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
	if err := rootCmd.ParseFlags(os.Args[1:]); err != nil {
		plog.Fatalln(err)
	}

	plog.SetVerbosity(verbosity - quietness)
	if err := rootCmd.Execute(); err != nil {
		plog.Fatalln(err)
	}
}
