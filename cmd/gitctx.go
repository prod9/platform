package cmd

import (
	"github.com/spf13/cobra"
	"platform.prodigy9.co/gitctx"
	"platform.prodigy9.co/internal/plog"
	"platform.prodigy9.co/project"
	"platform.prodigy9.co/releases"
)

var GitCtxCmd = &cobra.Command{
	Use:   "gitctx",
	Short: "Prints detected git information",
	Run:   runGitCtx,
}

func runGitCtx(cmd *cobra.Command, args []string) {
	cfg, err := project.Configure(".")
	if err != nil {
		plog.Fatalln(err)
	}

	git, err := gitctx.New(cfg)
	if err != nil {
		plog.Fatalln(err)
	}

	remote, err := git.MainRemoteName()
	if err != nil {
		plog.Fatalln(err)
	}

	branch, err := git.CurrentBranch()
	if err != nil {
		plog.Fatalln(err)
	}

	col, err := releases.Recover(cfg)
	if err != nil {
		plog.Fatalln(err)
	}

	strat, err := releases.FindStrategy(cfg.Strategy)
	if err != nil {
		plog.Fatalln(err)
	}

	plog.GitInfo("remote", remote)
	plog.GitInfo("branch", branch)
	latest := col.LatestName(strat)
	if latest != "" {
		plog.GitInfo("latest tag", latest)
	} else {
		plog.GitInfo("latest tag", "(n/a)")
	}
}
