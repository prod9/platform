package main

import (
	"log"
	"os"

	"fx.prodigy9.co/cmd/prompts"
	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/config"
	"platform.prodigy9.co/releases"
)

var ReleaseCmd = &cobra.Command{
	Use:   "release [name]",
	Short: "Create a new release with the given name.",
	Run:   runReleaseCmd,
}

var (
	releaseMajor bool
	releaseMinor bool
	releasePatch bool
)

func init() {
}

func runReleaseCmd(cmd *cobra.Command, args []string) {
	cfg, err := config.Configure(".")
	if err != nil {
		log.Fatalln(err)
	}

	strat, err := releases.FindStrategy(cfg.Strategy)
	if err != nil {
		log.Fatalln(err)
	}

	opts := &releases.Options{}
	if len(args) > 0 {
		opts.Name = args[0]
	}

	rel, err := strat.Generate(cfg, opts)
	if err != nil {
		log.Fatalln(err)
	}

	if err = toml.NewEncoder(os.Stdout).Encode(rel); err != nil {
		log.Fatalln(err)
	}

	sess := prompts.New(nil, nil)
	if !sess.YesNo("create this release?") {
		return
	}

	if err = strat.Create(cfg, rel); err != nil {
		log.Fatalln(err)
	}
}
