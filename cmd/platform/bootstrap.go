package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"

	"fx.prodigy9.co/cmd/prompts"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/bootstrapper"
)

var BootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Bootstraps the project for building with the platform app",
	Run:   runBootstrapCmd,
}

func runBootstrapCmd(cmd *cobra.Command, args []string) {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalln(err)
	}

	wd, err = filepath.Abs(wd)
	if err != nil {
		log.Fatalln(err)
	}

	sess := prompts.New(nil, nil)
	info := &bootstrapper.Info{
		ProjectName:     filepath.Base(wd),
		GoVersion:       runtime.Version()[2:],
		Maintainer:      sess.Str("your name"),
		MaintainerEmail: sess.Str("your email"),
	}
	if err := bootstrapper.Bootstrap(wd, info); err != nil {
		log.Fatalln(err)
	}
}
