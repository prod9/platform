// Package cmd wires the platform CLI: the root Cobra command, its persistent flags, and
// every subcommand. Single-file subcommands live in the package itself; a subcommand
// with its own file cluster gets a subpackage (cmd/init).
package cmd

import (
	fxcmd "fx.prodigy9.co/cmd"
	"github.com/spf13/cobra"
	initcmd "platform.prodigy9.co/cmd/init"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/internal/buildlog"
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

	rootCmd.PersistentPreRun = func(*cobra.Command, []string) {
		buildlog.SetVerbosity(verbosity - quietness)
	}

	rootCmd.PersistentFlags().StringVarP(&conf.PlatformFilename, "file", "f",
		conf.PlatformFilename, "specify a different platform.toml to load")

	rootCmd.AddCommand(
		initcmd.Cmd,
		BuildCmd,
		CleanCmd,
		ConfigureCmd,
		ExecCmd,
		ExportCmd,
		ListCmd,
		PreviewCmd,
		RenderCmd,
		PublishCmd,
		ReleaseCmd,
		VanityCmd,

		fxcmd.PrintConfigCmd,
	)
}

// Execute runs the root command; main defers to it.
func Execute() error {
	return rootCmd.Execute()
}
