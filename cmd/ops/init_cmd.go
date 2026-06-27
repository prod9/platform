package ops

import (
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"

	"fx.prodigy9.co/cmd/prompts"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/baseline"
	"platform.prodigy9.co/bootstrapper"
	"platform.prodigy9.co/internal/buildlog"
)

var initForce bool

var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise an infra repository: git init, baseline directives, platform.toml",
	Run:   runInitCmd,
}

func init() {
	InitCmd.Flags().BoolVar(&initForce, "force", false,
		"replace existing files instead of keeping them")
}

func runInitCmd(cmd *cobra.Command, args []string) {
	wd, err := os.Getwd()
	if err != nil {
		buildlog.Fatalln(err)
	}

	sess := prompts.New(nil, args)
	info := &bootstrapper.Info{
		ProjectName:     filepath.Base(wd),
		GoVersion:       runtime.Version()[2:],
		Maintainer:      sess.Str("your name"),
		MaintainerEmail: sess.Str("your email"),
		Repository:      sess.Str("github repository address (without https:// prefix)"),
		ImagePrefix:     sess.Str("docker image prefix (e.g. ghcr.io/prod9/)"),
		DefsModule:      baseline.DefsModule,
		DefsVersion:     baseline.DefsVersion,
	}

	// Greenfield only: a fresh infra repo needs a cue.mod so render can load its apps; an
	// existing module is the operator's truth and is left untouched, so don't prompt for it.
	if !bootstrapper.HasCueModule(wd) {
		info.ModulePath = sess.OptionalStr("cue module path", info.Repository)
	}

	files, err := baseline.EmbeddedFiles()
	if err != nil {
		buildlog.Fatalln(err)
	}

	vars := maps.Clone(baseline.DefaultVars)
	selected := selectComponents(sess, files)

	plan, err := bootstrapper.AnalyzeInit(wd, info, selected, vars)
	if err != nil {
		buildlog.Fatalln(err)
	}

	plan.Print(os.Stdout)
	if !sess.YesNo("apply this plan?") {
		return
	}

	replace := initForce
	if n := plan.Overwrites(); n > 0 && !replace {
		replace = sess.YesNo(fmt.Sprintf("replace %d existing file(s)?", n))
	}

	if err := ensureGitRepo(wd); err != nil {
		buildlog.Fatalln(err)
	}
	if err := plan.Apply(replace); err != nil {
		buildlog.Fatalln(err)
	}
	for _, f := range plan.Files {
		buildlog.File(f.Action.String(), f.Path)
	}
}

// selectComponents picks which built-in components to install: a checkbox list of every
// built-in file with Defaults pre-checked, driven interactively, by positional args, or
// defaulting under ALWAYS_YES. Returns the chosen subset of files (written into apps/).
func selectComponents(sess *prompts.Session, files map[string][]byte) map[string][]byte {
	names := fileNames(files)
	sort.Strings(names)
	chosen := sess.OptionalMultiSelect("install components", baseline.Defaults, names)

	selected := map[string][]byte{}
	for _, name := range chosen {
		if body, ok := files[name]; ok {
			selected[name] = body
		}
	}
	return selected
}

func fileNames(files map[string][]byte) []string {
	out := make([]string, 0, len(files))
	for n := range files {
		out = append(out, n)
	}
	return out
}

// ensureGitRepo runs `git init` when dir is not already inside a git work tree —
// `platform ops init` bootstraps a fresh infra repo, GitOps delivery needs one.
func ensureGitRepo(dir string) error {
	if bootstrapper.IsGitRepo(dir) {
		return nil
	}

	gitInit := exec.Command("git", "init", dir)
	gitInit.Stdout, gitInit.Stderr = os.Stdout, os.Stderr
	return gitInit.Run()
}
