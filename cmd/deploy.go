package cmd

import (
	// "log"

	"log"
	"os"

	"fx.prodigy9.co/cmd/prompts"
	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/builder"
	"platform.prodigy9.co/gitcmd"
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
		log.Fatalln(err)
	}
	if len(cfg.Environments) == 0 {
		log.Fatalln("no deploy environments defined, add some in project.toml")
	}

	strat, err := releases.FindStrategy(cfg.Strategy)
	if err != nil {
		log.Fatalln(err)
	}

	opts := &releases.Options{}
	rel, err := strat.Recover(cfg, opts)
	if err != nil {
		log.Fatalln(err)
	}

	sess := prompts.New(nil, args)
	targetEnv := sess.List("target environment", "", cfg.Environments)

	if err = toml.NewEncoder(os.Stdout).Encode(rel); err != nil {
		log.Fatalln(err)
	}
	if !sess.YesNo("deploy " + rel.Name + " to " + targetEnv + "?") {
		return
	}

	// build and publish image
	if !skipBuildOnDeploy {
		jobs, err := builder.JobsFromArgs(cfg, sess.Args())
		if err != nil {
			log.Fatalln(err)
		}

		for _, j := range jobs {
			j.Publish = true
			j.PublishImageName = j.ImageName + ":" + targetEnv
		}

		if err = builder.Build(cfg, jobs...); err != nil {
			log.Fatalln(err)
		}
	}

	if !skipTagOnDeploy {
		if _, err := gitcmd.TagF(cfg.ConfigDir, targetEnv); err != nil {
			log.Fatalln(err)
		} else if branch, err := gitcmd.CurrentBranch(cfg.ConfigDir); err != nil {
			log.Fatalln(err)
		} else if remote, err := gitcmd.TrackingRemote(cfg.ConfigDir, branch); err != nil {
			log.Fatalln(err)
		} else if _, err := gitcmd.PushTagF(cfg.ConfigDir, remote, targetEnv); err != nil {
			log.Fatalln(err)
		}
	}
}
