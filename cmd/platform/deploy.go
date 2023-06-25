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

var DeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a release",
	Run:   runDeploy,
}

func runDeploy(cmd *cobra.Command, args []string) {
	cfg, err := config.Configure(".")
	if err != nil {
		log.Fatalln(err)
	}
	if len(cfg.Environments) == 0 {
		log.Fatalln("no deploy environments defined, add some in project.toml")
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
	targetEnv := sess.List("target environment", "", cfg.Environments)

	maps.Keys(cfg.Modules)

	opts := &releases.Options{Name: releaseName}
	rel, err := strat.Recover(cfg, opts)
	if err != nil {
		log.Fatalln(err)
	}

	if err = toml.NewEncoder(os.Stdout).Encode(rel); err != nil {
		log.Fatalln(err)
	}
	if !sess.YesNo("deploy " + releaseName + " to " + targetEnv + "?") {
		return
	}

	jobs, err := builder.JobsFromArgs(cfg, sess.Args())
	if err != nil {
		log.Fatalln(err)
	}

	for _, j := range jobs {
		j.Publish = true
		j.PublishImageName = j.ImageName + ":" + targetEnv
	}

	// actually publish the image
	if err = builder.Build(cfg, jobs...); err != nil {
		log.Fatalln(err)
	} else if _, err := gitcmd.TagF(cfg.ConfigDir, targetEnv); err != nil {
		log.Fatalln(err)
	} else if branch, err := gitcmd.CurrentBranch(cfg.ConfigDir); err != nil {
		log.Fatalln(err)
	} else if remote, err := gitcmd.TrackingRemote(cfg.ConfigDir, branch); err != nil {
		log.Fatalln(err)
	} else if _, err := gitcmd.PushTagF(cfg.ConfigDir, remote, targetEnv); err != nil {
		log.Fatalln(err)
	}
}
