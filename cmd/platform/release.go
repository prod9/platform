package main

import (
	"github.com/spf13/cobra"
)

var ReleaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Create a new release",
	Run:   runReleaseCmd,
}

var (
	releaseMajor bool
	releaseMinor bool
	releasePatch bool
)

func init() {
	f := ReleaseCmd.Flags()
	f.BoolVar(&releaseMajor, "major", false, "Creates a major release (for semver releases)")
	f.BoolVar(&releaseMinor, "minor", false, "Creates a minor release (for semver releases)")
	f.BoolVar(&releasePatch, "patch", false, "Creates a patch release (for semver releases)")
	ReleaseCmd.MarkFlagsMutuallyExclusive("major", "minor", "patch")
}

func runReleaseCmd(cmd *cobra.Command, args []string) {

}
