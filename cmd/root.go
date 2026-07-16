package cmd

import (
	fxcmd "fx.prodigy9.co/cmd"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/cmd/build"
	"platform.prodigy9.co/cmd/clean"
	"platform.prodigy9.co/cmd/configure"
	"platform.prodigy9.co/cmd/exec"
	"platform.prodigy9.co/cmd/export"
	initcmd "platform.prodigy9.co/cmd/init"
	"platform.prodigy9.co/cmd/ls"
	"platform.prodigy9.co/cmd/preview"
	"platform.prodigy9.co/cmd/publish"
	"platform.prodigy9.co/cmd/release"
	"platform.prodigy9.co/cmd/render"
	"platform.prodigy9.co/cmd/vanity"
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
		initcmd.Cmd,
		build.Cmd,
		clean.Cmd,
		configure.Cmd,
		exec.Cmd,
		export.Cmd,
		ls.Cmd,
		preview.Cmd,
		render.Cmd,
		publish.Cmd,
		release.Cmd,
		vanity.Cmd,

		fxcmd.PrintConfigCmd,
	)
}

// Execute runs the root command; main defers to it.
func Execute() error {
	return rootCmd.Execute()
}
