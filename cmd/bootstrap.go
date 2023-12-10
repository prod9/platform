package cmd

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

	if len(args) > 0 {
		wd = filepath.Join(wd, args[0])
		args = args[1:]
	}

	wd, err = filepath.Abs(wd)
	if err != nil {
		log.Fatalln(err)
	}

	sess := prompts.New(nil, args)
	info := &bootstrapper.Info{
		ProjectName:     filepath.Base(wd),
		GoVersion:       runtime.Version()[2:],
		Maintainer:      sess.Str("your name"),
		MaintainerEmail: sess.Str("your email"),
		Repository:      sess.Str("github repository address (without https:// prefix)"),
		ImagePrefix:     sess.Str("docker image prefix (e.g. ghcr.io/prod9/)"),
	}
	if err := bootstrapper.Bootstrap(wd, info); err != nil {
		log.Fatalln(err)
	}
}
