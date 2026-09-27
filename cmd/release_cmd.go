package cmd

import (
	"errors"

	"fx.prodigy9.co/cmd/prompts"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/git"
	"platform.prodigy9.co/releases"
)

var (
	ReleaseCmd = &cobra.Command{
		Use:   "release [name]",
		Short: "Create a release by bumping or supplying a full SemVer name.",
		Args:  releaseArgs,
		RunE:  runReleaseCmd,
	}

	forceRelease bool

	bumpPatch bool
	bumpMinor bool
	bumpMajor bool
)

func init() {
	ReleaseCmd.Flags().BoolVar(&forceRelease, "force", false,
		"Force release even if worktree is dirty")

	ReleaseCmd.Flags().BoolVarP(&bumpPatch, "patch", "p", false,
		"(semver only) Increment the patch version or finalize the most recent prerelease")
	ReleaseCmd.Flags().BoolVarP(&bumpMinor, "minor", "m", false,
		"(semver only) Create new release by incrementing minor version from the most recent release")
	ReleaseCmd.Flags().BoolVar(&bumpMajor, "major", false,
		"(semver only) Create new release by incrementing major version from the most recent release")
}

func releaseArgs(cmd *cobra.Command, args []string) error {
	if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
		return err
	}
	flags := cmd.Flags()
	patch, minor, major := flags.Changed("patch"), flags.Changed("minor"), flags.Changed("major")
	if (patch && minor) || (patch && major) || (minor && major) {
		return errors.New("only one of --patch, --minor, or --major may be specified")
	}
	if len(args) == 1 && (patch || minor || major) {
		return errors.New("release name and bump flags are mutually exclusive")
	}
	return nil
}

func runReleaseCmd(cmd *cobra.Command, args []string) error {
	var request releases.Request = releases.BumpAny
	switch {
	case len(args) == 1:
		request = releases.Name(args[0])
	case bumpPatch:
		request = releases.BumpPatch
	case bumpMinor:
		request = releases.BumpMinor
	case bumpMajor:
		request = releases.BumpMajor
	}

	cfg, err := conf.Load(".")
	if err != nil {
		return err
	}

	g := git.New(cfg)
	opts := &releases.Options{Force: forceRelease, Request: request}

	rel, err := releases.Generate(cfg, g, opts)
	if err != nil {
		return err
	}

	rel.Changelog()
	sess := prompts.New(nil, nil)
	if !sess.YesNo("create this release?") {
		return nil
	}

	return releases.Create(cfg, g, rel)
}
