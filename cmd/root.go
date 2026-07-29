// Package cmd wires the platform CLI: the root Cobra command, its persistent flags, and
// every subcommand. Single-file subcommands live in the package itself; a subcommand
// with its own file cluster gets a subpackage (cmd/init).
package cmd

import (
	"errors"
	"runtime/debug"

	fxcmd "fx.prodigy9.co/cmd"
	"github.com/spf13/cobra"
	initcmd "platform.prodigy9.co/cmd/init"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/internal/termlog"
)

var rootCmd = &cobra.Command{
	Use:     "platform",
	Short:   "PRODIGY9 platform swiss army knife",
	Version: versionString(debug.ReadBuildInfo()),

	// Execute is the one reporter of a failure, and usage text is for a malformed
	// invocation rather than for a build that failed on its tenth step.
	SilenceErrors: true,
	SilenceUsage:  true,
}

var (
	quietness int
	verbosity int
)

func init() {
	rootCmd.PersistentFlags().CountVarP(&quietness, "quiet", "q", "less verbose logging")
	rootCmd.PersistentFlags().CountVarP(&verbosity, "verbose", "v", "more verbose logging")
	rootCmd.SetVersionTemplate("{{.Version}}\n")

	rootCmd.PersistentPreRun = func(*cobra.Command, []string) {
		termlog.SetVerbosity(verbosity - quietness)
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
		ServeCmd,
		VanityCmd,
		VersionsCmd,

		fxcmd.PrintConfigCmd,
	)
}

// Execute runs the root command and reports the process's exit code, having already
// reported whatever went wrong. It is the single place a platform invocation decides how it
// ended: commands return, so their deferred cleanup — an engine session above all — runs
// before the process goes anywhere near os.Exit.
//
// A command that must reproduce a child's status returns an exitError saying so; everything
// else that fails is a plain 1.
func Execute() int {
	err := rootCmd.Execute()
	if err == nil {
		return 0
	}

	var exit exitError
	if errors.As(err, &exit) {
		return exit.code
	}

	termlog.Error(err)
	return 1
}
