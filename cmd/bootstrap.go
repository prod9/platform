package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"fx.prodigy9.co/cmd/prompts"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/internal/buildlog"
	"platform.prodigy9.co/scaffold"
)

var bootstrapForce bool

var BootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Bootstraps the project for building with the platform app",
	Run:   runBootstrapCmd,
}

func init() {
	BootstrapCmd.Flags().BoolVar(&bootstrapForce, "force", false,
		"replace existing files instead of keeping them")
}

func runBootstrapCmd(cmd *cobra.Command, args []string) {
	wd, err := os.Getwd()
	if err != nil {
		buildlog.Fatalln(err)
	}

	sess := prompts.New(nil, args)
	info := &scaffold.Info{
		ProjectName:     filepath.Base(wd),
		GoVersion:       runtime.Version()[2:],
		Maintainer:      sess.Str("your name"),
		MaintainerEmail: sess.Str("your email"),
		Repository:      sess.Str("github repository address (without https:// prefix)"),
		ImagePrefix:     sess.Str("docker image prefix (e.g. ghcr.io/prod9/)"),
	}

	plan, err := scaffold.Analyze(wd, info, nil)
	if err != nil {
		buildlog.Fatalln(err)
	}

	plan.Print(os.Stdout)
	if !sess.YesNo("apply this plan?") {
		return
	}

	replace := bootstrapForce
	if n := plan.Overwrites(); n > 0 && !replace {
		replace = sess.YesNo(fmt.Sprintf("replace %d existing file(s)?", n))
	}

	apply := plan.Apply
	if replace {
		apply = plan.ApplyOverwrite
	}
	if err := apply(); err != nil {
		buildlog.Fatalln(err)
	}
	for _, f := range plan.Files {
		buildlog.File(f.Action.String(), f.Path)
	}
}
