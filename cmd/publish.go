package cmd

import (
	"context"
	"os"

	"fx.prodigy9.co/cmd/prompts"
	fxconfig "fx.prodigy9.co/config"
	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/engine"
	"platform.prodigy9.co/gitctx"
	"platform.prodigy9.co/internal/buildlog"
	"platform.prodigy9.co/project"
	"platform.prodigy9.co/releases"
)

var PublishCmd = &cobra.Command{
	Use:   "publish [modules...]",
	Short: "Builds current directory and publish as a release",
	Run:   runPublish,
}

func runPublish(cmd *cobra.Command, args []string) {
	cfg, err := project.Configure(".")
	if err != nil {
		buildlog.Fatalln(err)
	}

	strat, err := releases.FindStrategy(cfg.Strategy)
	if err != nil {
		buildlog.Fatalln(err)
	}

	git := gitctx.New(cfg)

	collection, err := releases.Recover(cfg, git)
	if err != nil {
		buildlog.Fatalln(err)
	}

	rel, err := collection.GetLatest(git, strat)
	if err != nil {
		buildlog.Fatalln(err)
	}

	p := prompts.New(nil, args)
	if err = toml.NewEncoder(os.Stdout).Encode(rel); err != nil {
		buildlog.Fatalln(err)
	}
	if !p.YesNo("publish " + rel.Name + "?") {
		return
	}

	eng := engine.New(fxconfig.Configure())
	defer eng.Close()
	ctx := engine.NewContext(context.Background(), eng)

	if err := engine.BuildAndPublish(ctx, cfg, p.Args(), rel.Name); err != nil {
		buildlog.Fatalln(err)
	}
}
