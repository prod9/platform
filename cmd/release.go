package cmd

import (
	"errors"

	"fx.prodigy9.co/cmd/prompts"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/git"
	"platform.prodigy9.co/internal/buildlog"
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
	ReleaseCmd.Flags().BoolVar(&forceRelease, "force", false,
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
		buildlog.Fatalln(errors.New("only one of --patch, --minor, or --major may be specified"))
	}

	opts := &releases.Options{Force: forceRelease}
	switch {
	case bumpPatch:
		opts.Bump = releases.BumpPatch
	case bumpMinor:
		opts.Bump = releases.BumpMinor
	case bumpMajor:
		opts.Bump = releases.BumpMajor
	default:
		opts.Bump = releases.BumpAny
	}

	cfg, err := project.Configure(".")
	if err != nil {
		buildlog.Fatalln(err)
	}

	g := git.New(cfg)

	rel, err := releases.Generate(cfg, g, opts)
	if err != nil {
		buildlog.Fatalln(err)
	}

	rel.Changelog()
	sess := prompts.New(nil, nil)
	if !sess.YesNo("create this release?") {
		return
	}

	if err = releases.Create(cfg, g, rel); err != nil {
		buildlog.Fatalln(err)
	}
}
