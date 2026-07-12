package cmd

import (
	"fmt"
	"os"

	"fx.prodigy9.co/cmd/prompts"
	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/internal/buildlog"
	"platform.prodigy9.co/project"
)

var initForce bool

var InitCmd = &cobra.Command{
	Use:     "init",
	Aliases: []string{"scaffold"},
	Short:   "Scaffold a repo from its discovered framework (platform.toml + launcher + the framework's own files)",
	Run:     runInit,
}

func init() {
	InitCmd.Flags().BoolVar(&initForce, "force", false,
		"replace existing files instead of keeping them")
}

// runInit is one path for every repo: gather the operator inputs common to all, then let the
// scaffold driver discover the framework and compute the plan. What a repo gets is entirely
// the framework's Scaffold contribution — never an app-vs-infra branch here.
func runInit(cmd *cobra.Command, args []string) {
	wd, err := os.Getwd()
	if err != nil {
		buildlog.Fatalln(err)
	}
	sess := prompts.New(nil, args)

	info := &Info{
		Maintainer:      sess.Str("your name"),
		MaintainerEmail: sess.Str("your email"),
		Repository:      sess.Str("github repository address (without https:// prefix)"),
	}

	plan, err := Analyze(wd, info)
	if err != nil {
		buildlog.Fatalln(err)
	}
	applyPlan(wd, sess, plan)
}

// applyPlan is the shared tail: show the plan, confirm, ensure a git repo, write the files,
// then print the effective parsed config so the operator sees the resolved result in one shot.
func applyPlan(wd string, sess *prompts.Session, plan *Plan) {
	plan.Print(os.Stdout)
	if !sess.YesNo("apply this plan?") {
		return
	}

	replace := initForce
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
		if f.Action == FileOverwrite && !replace {
			continue // Apply kept the existing file — no action taken
		}
		buildlog.File(f.Action.String(), f.Path)
	}

	// Close with the effective parsed config (same view as `configure`) so the operator sees
	// the resolved result of the freshly written platform.toml.
	cfg, err := project.Configure(wd)
	if err != nil {
		buildlog.Fatalln(err)
	}
	if err := toml.NewEncoder(os.Stdout).Encode(cfg); err != nil {
		buildlog.Fatalln(err)
	}
}
