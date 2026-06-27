package cmd

import (
	"context"
	"os"

	"fx.prodigy9.co/cmd/prompts"
	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/builder"
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

	attempt, err := builder.AttemptFrom(cfg, p.Args(), builder.PublishBuild)
	if err != nil {
		buildlog.Fatalln(err)
	}

	ctx := context.Background()
	sess, err := engine.New(ctx)
	if err != nil {
		buildlog.Fatalln(err)
	}
	defer sess.Close()

	// Tag the image with the release name
	for _, unit := range attempt.Units {
		unit.ImageName = unit.ImageName + ":" + rel.Name
	}

	builds, err := engine.Build(sess, attempt)
	if err != nil {
		buildlog.Fatalln(err)
	}
	results, err := engine.Publish(sess, builds...)
	if err != nil {
		buildlog.Fatalln(err)
	}

	for _, result := range results {
		if result.Err != nil {
			buildlog.Error(result.Err)
		}
	}
}
