package cmd

import (
	"context"
	"errors"
	"os"

	"fx.prodigy9.co/cmd/prompts"
	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/builder"
	"platform.prodigy9.co/gitctx"
	"platform.prodigy9.co/internal/buildlog"
	"platform.prodigy9.co/project"
	"platform.prodigy9.co/releases"
)

var DeployCmd = &cobra.Command{
	Use:   "deploy (release)",
	Short: "Deploy a release",
	Run:   runDeploy,
}

var (
	skipBuildOnDeploy bool
	skipTagOnDeploy   bool
)

func init() {
	// TODO: Document how to use these
	//  * uses deploy -n on local machine to create tag
	//  * uses deploy -b on remote machines to skip tag, just builds
	DeployCmd.Flags().BoolVarP(&skipBuildOnDeploy, "no-build", "n", false,
		"Skips building, only create tags.")
	DeployCmd.Flags().BoolVarP(&skipTagOnDeploy, "no-tag", "b", false,
		"Only builds, do not create tags.")
}

func runDeploy(cmd *cobra.Command, args []string) {
	cfg, err := project.Configure(".")
	if err != nil {
		buildlog.Fatalln(err)
	}
	if len(cfg.Environments) == 0 {
		buildlog.Fatalln(errors.New("no deploy environments defined, add some in project.toml"))
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
	targetEnv := p.List("target environment", "", cfg.Environments)
	if err = toml.NewEncoder(os.Stdout).Encode(rel); err != nil {
		buildlog.Fatalln(err)
	}
	if !p.YesNo("deploy " + rel.Name + " to " + targetEnv + "?") {
		return
	}

	// build and publish image
	if !skipBuildOnDeploy {
		sess, err := builder.NewSession(context.Background())
		if err != nil {
			buildlog.Fatalln(err)
		}
		defer sess.Close()

		attempt, err := builder.AttemptFrom(cfg, p.Args(), builder.PublishBuild)
		if err != nil {
			buildlog.Fatalln(err)
		}

		builds, err := builder.Build(sess, attempt)
		if err != nil {
			buildlog.Fatalln(err)
		}

		for _, unit := range attempt.Units {
			unit.ImageName = unit.ImageName + ":" + targetEnv
		}

		results, err := builder.Publish(sess, builds...)
		if err != nil {
			buildlog.Fatalln(err)
		}

		anyErr := false
		for _, result := range results {
			if result.Err != nil {
				buildlog.Error(result.Err)
				anyErr = true
			}
		}

		if anyErr {
			os.Exit(1)
		}
	}

	if !skipTagOnDeploy {
		if _, err := git.SetEnvironmentTag(targetEnv); err != nil {
			buildlog.Fatalln(err)
		} else if err := git.PushEnvironmentTag(targetEnv); err != nil {
			buildlog.Fatalln(err)
		}
	}
}
