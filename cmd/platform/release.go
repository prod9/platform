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
	Use:   "release",
	Short: "Create a new release",
	Run:   runReleaseCmd,
}

var (
	releaseMajor bool
	releaseMinor bool
	releasePatch bool
)

func init() {
	f := ReleaseCmd.Flags()
	f.BoolVar(&releaseMajor, "major", false, "Creates a major release (for semver releases)")
	f.BoolVar(&releaseMinor, "minor", false, "Creates a minor release (for semver releases)")
	f.BoolVar(&releasePatch, "patch", false, "Creates a patch release (for semver releases)")
	ReleaseCmd.MarkFlagsMutuallyExclusive("major", "minor", "patch")
}

func runReleaseCmd(cmd *cobra.Command, args []string) {
	// TODO: Check major minor patch

	cfg, err := config.Configure(".")
	if err != nil {
		log.Fatalln(err)
	}

	strat, err := releases.FindStrategy(cfg.Strategy)
	if err != nil {
		log.Fatalln(err)
	}

	rel, err := strat.Generate(cfg, &releases.Options{
		IncrementMajor: releaseMajor,
		IncrementMinor: releaseMinor,
		IncrementPatch: releasePatch,
	})
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
