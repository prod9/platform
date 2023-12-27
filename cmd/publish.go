package cmd

import (
	"context"
	"os"

	"fx.prodigy9.co/cmd/prompts"
	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/builder"
	"platform.prodigy9.co/internal/plog"
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
		plog.Fatalln(err)
	}

	strat, err := releases.FindStrategy(cfg.Strategy)
	if err != nil {
		plog.Fatalln(err)
	}

	opts := &releases.Options{}
	rel, err := strat.Recover(cfg, opts)
	if err != nil {
		plog.Fatalln(err)
	}

	p := prompts.New(nil, args)
	if err = toml.NewEncoder(os.Stdout).Encode(rel); err != nil {
		plog.Fatalln(err)
	}
	if !p.YesNo("publish " + rel.Name + "?") {
		return
	}

	jobs, err := builder.JobsFromArgs(cfg, p.Args())
	if err != nil {
		plog.Fatalln(err)
	}

	ctx := context.Background()
	sess, err := builder.NewSession(ctx)
	if err != nil {
		plog.Fatalln(err)
	}
	defer sess.Close()

	builds, err := builder.Build(sess, jobs...)
	if err != nil {
		plog.Fatalln(err)
	}
	results, err := builder.Publish(sess, builds...)
	if err != nil {
		plog.Fatalln(err)
	}

	for _, result := range results {
		if result.Err != nil {
			plog.Error(result.Err)
		}
	}
}
