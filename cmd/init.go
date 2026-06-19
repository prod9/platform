package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"fx.prodigy9.co/cmd/prompts"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/bootstrapper"
	"platform.prodigy9.co/core/baseline"
	"platform.prodigy9.co/internal/plog"
)

var initForce bool

var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise an infra repository: git init, baseline directives, platform.toml",
	Run:   runInitCmd,
}

func init() {
	InitCmd.Flags().BoolVar(&initForce, "force", false,
		"apply the init plan without confirming (CI / non-interactive)")
}

func runInitCmd(cmd *cobra.Command, args []string) {
	wd, err := os.Getwd()
	if err != nil {
		plog.Fatalln(err)
	}

	sess := prompts.New(nil, args)
	info := &bootstrapper.Info{
		ProjectName:     filepath.Base(wd),
		GoVersion:       runtime.Version()[2:],
		Maintainer:      sess.Str("your name"),
		MaintainerEmail: sess.Str("your email"),
		Repository:      sess.Str("github repository address (without https:// prefix)"),
		ImagePrefix:     sess.Str("docker image prefix (e.g. ghcr.io/prod9/)"),
	}

	files, err := baseline.EmbeddedFiles()
	if err != nil {
		plog.Fatalln(err)
	}

	plan, err := bootstrapper.AnalyzeInit(wd, info, files, baseline.DefaultVars)
	if err != nil {
		plog.Fatalln(err)
	}

	plan.Print(os.Stdout)
	if !initForce && !sess.YesNo("apply this plan?") {
		return
	}

	if err := ensureGitRepo(wd); err != nil {
		plog.Fatalln(err)
	}
	if err := plan.Apply(); err != nil {
		plog.Fatalln(err)
	}
	for _, f := range plan.Files {
		plog.File(f.Action.String(), f.Path)
	}
}

// ensureGitRepo runs `git init` when dir is not already inside a git work tree —
// `platform init` bootstraps a fresh infra repo, GitOps delivery needs one.
func ensureGitRepo(dir string) error {
	if bootstrapper.IsGitRepo(dir) {
		return nil
	}

	gitInit := exec.Command("git", "init", dir)
	gitInit.Stdout, gitInit.Stderr = os.Stdout, os.Stderr
	return gitInit.Run()
}
