package main

import (
	// "log"

	"log"
	"os"

	"fx.prodigy9.co/cmd/prompts"
	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
	"platform.prodigy9.co/builder"
	"platform.prodigy9.co/config"
	"platform.prodigy9.co/gitcmd"
	"platform.prodigy9.co/releases"
)

var PublishCmd = &cobra.Command{
	Use:   "publish (release)",
	Short: "Builds and publish a release",
	Run:   runPublish,
}

func runPublish(cmd *cobra.Command, args []string) {
	cfg, err := config.Configure(".")
	if err != nil {
		log.Fatalln(err)
	}

	strat, err := releases.FindStrategy(cfg.Strategy)
	if err != nil {
		log.Fatalln(err)
	}

	allReleases, err := strat.List(cfg)
	if err != nil {
		log.Fatalln(err)
	}
	if len(allReleases) == 0 {
		log.Fatalln("no release to deploy, create some first.")
	}

	sess := prompts.New(nil, args)
	releaseName := sess.Str("which release")

	opts := &releases.Options{Name: releaseName}
	rel, err := strat.Recover(cfg, opts)
	if err != nil {
		log.Fatalln(err)
	}

	if err = toml.NewEncoder(os.Stdout).Encode(rel); err != nil {
		log.Fatalln(err)
	}
	if !sess.YesNo("publish " + releaseName + "?") {
		return
	}

	jobs, err := builder.JobsFromArgs(cfg, sess.Args())
	if err != nil {
		log.Fatalln(err)
	}

	for _, j := range jobs {
		j.Publish = true
		j.PublishImageName = j.ImageName + ":" + rel.Name
	}

	if err = builder.Build(cfg, jobs...); err != nil {
		log.Fatalln(err)
	}
}
