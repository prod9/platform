package ops

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"

	"fx.prodigy9.co/cmd/prompts"
	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/baseline"
	"platform.prodigy9.co/bootstrapper"
	"platform.prodigy9.co/internal/buildlog"
	"platform.prodigy9.co/project"
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

	daggerVersion := baseline.DaggerVersion()
	if daggerVersion == "" {
		buildlog.Fatalln(errors.New("could not determine the linked dagger SDK version"))
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

	// The module path feeds both the cue.mod scaffold and the `<module>/defaults` import in
	// templated apps. A fresh repo prompts for it (and gets a scaffolded cue.mod); an existing
	// module is the operator's truth — read its path, leave the module file untouched.
	if bootstrapper.HasCueModule(wd) {
		info.ModulePath, err = bootstrapper.ModulePath(wd)
		if err != nil {
			buildlog.Fatalln(err)
		}
	} else {
		info.ModulePath = sess.OptionalStr("cue module path", info.Repository)
	}

	data := baseline.TemplateData{
		DaggerVersion:    daggerVersion,
		RegistryUsername: sess.Str("registry username"),
		RegistryPassword: sess.SensitiveStr("registry password"),
		ModulePath:       info.ModulePath,
		OpsImage:         project.InferOpsImage(info.Repository),
	}

	files, err := baseline.EmbeddedFiles()
	if err != nil {
		buildlog.Fatalln(err)
	}

	rendered, err := baseline.Render(selectComponents(sess, files), data)
	if err != nil {
		buildlog.Fatalln(err)
	}

	vars := maps.Clone(baseline.DefaultVars)
	plan, err := bootstrapper.AnalyzeInit(wd, info, vars)
	if err != nil {
		buildlog.Fatalln(err)
	}
	for _, f := range rendered {
		plan.AddFile(f.Path, f.Body, 0644)
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
	apply := plan.Apply
	if replace {
		apply = plan.ApplyOverwrite
	}
	if err := apply(); err != nil {
		buildlog.Fatalln(err)
	}
	for _, f := range plan.Files {
		buildlog.File(f.Action.String(), f.Path)
	}

	// Close with the effective parsed config (same view as `configure`) so the operator
	// sees the resolved result of the freshly written platform.toml in one shot.
	cfg, err := project.Configure(wd)
	if err != nil {
		buildlog.Fatalln(err)
	}
	if err := toml.NewEncoder(os.Stdout).Encode(cfg); err != nil {
		buildlog.Fatalln(err)
	}
}

// selectComponents picks which built-in components to install: a checkbox list of the
// optional files with Defaults pre-checked, driven interactively, by positional args, or
// defaulting under ALWAYS_YES. Mandatory files (the shared defaults/ package) are never
// offered — they are always included. Returns the files to install, keyed by name.
func selectComponents(sess *prompts.Session, files map[string][]byte) map[string][]byte {
	mandatory := map[string]bool{}
	for _, name := range baseline.Mandatory {
		mandatory[name] = true
	}

	optional := make([]string, 0, len(files))
	for name := range files {
		if !mandatory[name] {
			optional = append(optional, name)
		}
	}
	sort.Strings(optional)

	chosen := sess.OptionalMultiSelect("install components", baseline.Defaults, optional)

	selected := map[string][]byte{}
	for _, name := range append(chosen, baseline.Mandatory...) {
		if body, ok := files[name]; ok {
			selected[name] = body
		}
	}
	return selected
}

// ensureGitRepo runs `git init` when dir is not already its own git repo root —
// `platform ops init` bootstraps a standalone infra repo (GitOps delivery needs
// one), even when the target sits nested inside another checkout.
func ensureGitRepo(dir string) error {
	if bootstrapper.IsGitRoot(dir) {
		return nil
	}

	gitInit := exec.Command("git", "init", dir)
	gitInit.Stdout, gitInit.Stderr = os.Stdout, os.Stderr
	return gitInit.Run()
}
