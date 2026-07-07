package main

import (
	fxcmd "fx.prodigy9.co/cmd"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/cmd"
	"platform.prodigy9.co/cmd/ops"
	"platform.prodigy9.co/internal/buildlog"
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

	rootCmd.PersistentPreRun = func(*cobra.Command, []string) {
		buildlog.SetVerbosity(verbosity - quietness)
	}

	rootCmd.PersistentFlags().StringVarP(&project.PlatformFilename, "file", "f",
		project.PlatformFilename, "specify a different platform.toml to load")

	rootCmd.AddCommand(
		cmd.InitCmd,
		cmd.BuildCmd,
		cmd.ConfigureCmd,
		cmd.ExecCmd,
		cmd.ExportCmd,
		cmd.ListCmd,
		ops.Cmd,
		cmd.PreviewCmd,
		cmd.PublishCmd,
		cmd.ReleaseCmd,
		cmd.VanityCmd,

		fxcmd.PrintConfigCmd,
	)
}

func main() {
	defer buildlog.Event("exited")
	if err := rootCmd.Execute(); err != nil {
		buildlog.Fatalln(err)
	}
}
