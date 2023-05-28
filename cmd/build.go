package cmd

import (
	"log"

	"github.com/spf13/cobra"
	"platform.prodigy9.co/build"
)

var BuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Builds current directory",
	Run:   runBuild,
}

func runBuild(cmd *cobra.Command, args []string) {
	if err := build.Build("."); err != nil {
		log.Fatalln(err)
	}
}
