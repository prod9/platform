package cmd

import (
	"fmt"
	"os"

	"fx.prodigy9.co/cmd/prompts"
	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/internal/termlog"
	"platform.prodigy9.co/scaffolding"
)

var (
	InitCmd = &cobra.Command{
		Use:     "init",
		Aliases: []string{"scaffold"},
		Short:   "Scaffold a repo from its discovered framework (platform.toml + launcher + the framework's own files)",
		Run:     runInitCmd,
	}

	initForce bool
)

func init() {
	InitCmd.Flags().BoolVar(&initForce, "force", false,
		"replace existing files instead of keeping them")
}

func runInitCmd(_ *cobra.Command, args []string) {
	wd, err := os.Getwd()
	if err != nil {
		termlog.Fatalln(err)
	}
	target, err := scaffolding.Discover(wd)
	if err != nil {
		termlog.Fatalln(err)
	}
	sess := prompts.New(nil, args)

	info := scaffolding.Info{
		Maintainer:      sess.Str("your name"),
		MaintainerEmail: sess.Str("your email"),
		Repository:      sess.Str("github repository address (without https:// prefix)"),
	}

	scaffoldVars := target.ScaffoldVars()
	vars := make(map[string]string, len(scaffoldVars))
	for _, name := range scaffoldVars {
		vars[name] = sess.Str(name)
	}

	plan, err := target.Plan(info, vars)
	if err != nil {
		termlog.Fatalln(err)
	}

	applyInitPlan(wd, sess, plan)
}

func applyInitPlan(wd string, sess *prompts.Session, plan *scaffolding.Plan) {
	plan.Print(os.Stdout)
	if !sess.YesNo("apply this plan?") {
		return
	}

	replace := initForce
	if n := plan.Overwrites(); n > 0 && !replace {
		replace = sess.YesNo(fmt.Sprintf("replace %d existing file(s)?", n))
	}

	var applied []scaffolding.FileChange
	var err error
	if replace {
		applied, err = plan.ForceApply()
	} else {
		applied, err = plan.Apply()
	}
	if err != nil {
		termlog.Fatalln(err)
	}

	for _, file := range applied {
		termlog.File(file.Action.String(), file.Path)
	}

	cfg, err := conf.Load(wd)
	if err != nil {
		termlog.Fatalln(err)
	}
	if err := toml.NewEncoder(os.Stdout).Encode(cfg); err != nil {
		termlog.Fatalln(err)
	}
}
