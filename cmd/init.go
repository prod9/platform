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

// pickOptions records the operator's baseline selections into vars. Choices pick one
// variant each (List); every overlay toggle folds into a single pre-checked MultiSelect
// so the operator sees all optional components at once with the shipped defaults already
// ticked. Generic over ScanOptions — adding a baseline option file needs no change here.
func pickOptions(sess *prompts.Session, opts []baseline.Option, vars map[string]any) {
	var toggles []baseline.Option
	for _, opt := range opts {
		switch opt.Kind {
		case baseline.OptionChoice:
			current := opt.Default
			if v, ok := vars[opt.Key]; ok {
				current = fmt.Sprint(v)
			}
			vars[opt.Key] = sess.List(opt.Key, current, opt.Variants)

		case baseline.OptionToggle:
			toggles = append(toggles, opt)
		}
	}

	pickToggles(sess, toggles, vars)
}

// pickToggles folds the overlay toggles into one checkbox prompt, pre-checking the ones
// currently enabled in vars (the shipped defaults), then writes the string-bool result back.
func pickToggles(sess *prompts.Session, toggles []baseline.Option, vars map[string]any) {
	if len(toggles) == 0 {
		return
	}

	keys := make([]string, 0, len(toggles))
	defaults := make([]string, 0, len(toggles))
	for _, opt := range toggles {
		keys = append(keys, opt.Key)
		if fmt.Sprint(vars[opt.Key]) == "true" {
			defaults = append(defaults, opt.Key)
		}
	}

	enabled := map[string]bool{}
	for _, key := range sess.MultiSelect("enable optional components", keys, defaults) {
		enabled[key] = true
	}
	for _, opt := range toggles {
		if enabled[opt.Key] {
			vars[opt.Key] = "true"
		} else {
			vars[opt.Key] = "false"
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
