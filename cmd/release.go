package cmd

import (
	"errors"

	"fx.prodigy9.co/cmd/prompts"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/internal/plog"
	"platform.prodigy9.co/project"
	"platform.prodigy9.co/releases"
)

var ReleaseCmd = &cobra.Command{
	Use:   "release (name)",
	Short: "Create a new release with the given name.",
	Run:   runReleaseCmd,
}

var (
	forceRelease bool

	bumpPatch bool
	bumpMinor bool
	bumpMajor bool
)

func init() {
	ReleaseCmd.Flags().BoolVarP(&forceRelease, "force", "f", false,
		"Force release even if worktree is dirty")

	ReleaseCmd.Flags().BoolVarP(&bumpPatch, "patch", "p", false,
		"(semver only) Create new release by incrementing patch version from the most recent release")
	ReleaseCmd.Flags().BoolVarP(&bumpMinor, "minor", "m", false,
		"(semver only) Create new release by incrementing minor version from the most recent release")
	ReleaseCmd.Flags().BoolVar(&bumpMajor, "major", false,
		"(semver only) Create new release by incrementing major version from the most recent release")
}

func runReleaseCmd(cmd *cobra.Command, args []string) {
	if (bumpPatch && bumpMinor) ||
		(bumpPatch && bumpMajor) ||
		(bumpMinor && bumpMajor) {
		plog.Fatalln(errors.New("only one of --patch, --minor, or --major may be specified"))
	}

	cfg, err := project.Configure(".")
	if err != nil {
		plog.Fatalln(err)
	}

	strat, err := releases.FindStrategy(cfg.Strategy)
	if err != nil {
		plog.Fatalln(err)
	}

	opts := &releases.Options{Force: forceRelease}
	if len(args) > 0 {
		opts.Name = args[0]
	} else {
		switch {
		case bumpPatch:
			opts.Name, err = strat.NextName(cfg, releases.NamePatch)
		case bumpMinor:
			opts.Name, err = strat.NextName(cfg, releases.NameMinor)
		case bumpMajor:
			opts.Name, err = strat.NextName(cfg, releases.NameMajor)
		default:
			opts.Name, err = strat.NextName(cfg, releases.NameAny)
		}
		if err != nil {
			plog.Fatalln(err)
		}
	}

	rel, err := strat.Generate(cfg, opts)
	if err != nil {
		plog.Fatalln(err)
	}

	if err := rel.Render(); err != nil {
		plog.Fatalln(err)
	}
	sess := prompts.New(nil, nil)
	if !sess.YesNo("create this release?") {
		return
	}

	if err = strat.Create(cfg, rel); err != nil {
		plog.Fatalln(err)
	}
}
