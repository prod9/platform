package publish

import (
	"context"
	"os"

	"fx.prodigy9.co/cmd/prompts"
	fxconfig "fx.prodigy9.co/config"
	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/engine"
	"platform.prodigy9.co/git"
	"platform.prodigy9.co/internal/buildlog"
	"platform.prodigy9.co/project"
	"platform.prodigy9.co/releases"
)

var Cmd = &cobra.Command{
	Use:   "publish [modules...]",
	Short: "Builds current directory and publish as a release",
	Run:   run,
}

func run(cmd *cobra.Command, args []string) {
	cfg, err := project.Configure(".")
	if err != nil {
		buildlog.Fatalln(err)
	}

	strat, err := releases.FindStrategy(cfg.Strategy)
	if err != nil {
		buildlog.Fatalln(err)
	}

	// Versioned strategies publish the latest git-tagged release; a non-versioned one
	// (such as Rolling) has a single constant name and no tag to look up — publishing is
	// the deploy.
	name := ""
	if strat.IsVersioned() {
		g := git.New(cfg)
		collection, err := releases.Recover(cfg, g)
		if err != nil {
			buildlog.Fatalln(err)
		}
		rel, err := collection.GetLatest(g, strat)
		if err != nil {
			buildlog.Fatalln(err)
		}
		if err = toml.NewEncoder(os.Stdout).Encode(rel); err != nil {
			buildlog.Fatalln(err)
		}
		name = rel.Name
	} else if name, err = strat.NextName("", releases.BumpAny); err != nil {
		buildlog.Fatalln(err)
	}

	p := prompts.New(nil, args)
	if !p.YesNo("publish " + name + "?") {
		return
	}

	eng := engine.New(fxconfig.Configure())
	defer eng.Close()
	ctx := engine.NewContext(context.Background(), eng)

	if err := engine.BuildAndPublish(ctx, cfg, p.Args(), name); err != nil {
		buildlog.Fatalln(err)
	}
}
