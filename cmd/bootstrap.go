package cmd

import (
	"os"
	"path/filepath"
	"runtime"

	"fx.prodigy9.co/cmd/prompts"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/bootstrapper"
	"platform.prodigy9.co/internal/plog"
)

var bootstrapForce bool

var BootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Bootstraps the project for building with the platform app",
	Run:   runBootstrapCmd,
}

func init() {
	BootstrapCmd.Flags().BoolVar(&bootstrapForce, "force", false,
		"apply the bootstrap plan without confirming (CI / non-interactive)")
}

func runBootstrapCmd(cmd *cobra.Command, args []string) {
	wd, err := os.Getwd()
	if err != nil {
		plog.Fatalln(err)
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

	plan, err := bootstrapper.Analyze(wd, info, nil)
	if err != nil {
		plog.Fatalln(err)
	}

	plan.Print(os.Stdout)
	if !bootstrapForce && !sess.YesNo("apply this plan?") {
		return
	}

	if err := plan.Apply(); err != nil {
		plog.Fatalln(err)
	}
	for _, f := range plan.Files {
		plog.File(f.Action.String(), f.Path)
	}
}
