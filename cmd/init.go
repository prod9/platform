package cmd

import (
	"fmt"
	"maps"
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
	apps, err := baseline.EmbeddedApps()
	if err != nil {
		plog.Fatalln(err)
	}

	// Pick the optional baseline components into [ops.vars]; --force keeps the
	// shipped defaults (NGF-experimental on, argocd off) for CI.
	vars := maps.Clone(baseline.DefaultVars)
	if !initForce {
		pickOptions(sess, baseline.ScanOptions(fileNames(files)), vars)
	}

	plan, err := bootstrapper.AnalyzeInit(wd, info, files, apps, vars)
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

// pickOptions presents each baseline option as a picker entry and records the
// operator's selection into vars. Toggles are a yes/no checkbox pre-set from the
// current value; choices pick one variant. Generic over ScanOptions — adding a
// baseline option file needs no change here.
func pickOptions(sess *prompts.Session, opts []baseline.Option, vars map[string]any) {
	for _, opt := range opts {
		switch opt.Kind {
		case baseline.OptionToggle:
			checked := "no"
			if fmt.Sprint(vars[opt.Key]) == "true" {
				checked = "yes"
			}
			if sess.List("enable "+opt.Key+"?", checked, []string{"yes", "no"}) == "yes" {
				vars[opt.Key] = "true"
			} else {
				vars[opt.Key] = "false"
			}

		case baseline.OptionChoice:
			current := opt.Default
			if v, ok := vars[opt.Key]; ok {
				current = fmt.Sprint(v)
			}
			vars[opt.Key] = sess.List(opt.Key, current, opt.Variants)
		}
	}
}

func fileNames(files map[string][]byte) []string {
	out := make([]string, 0, len(files))
	for n := range files {
		out = append(out, n)
	}
	return out
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
