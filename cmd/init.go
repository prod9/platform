package cmd

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
	"platform.prodigy9.co/builder"
	"platform.prodigy9.co/internal/buildlog"
	"platform.prodigy9.co/project"
	"platform.prodigy9.co/scaffold"
)

var initForce bool

var InitCmd = &cobra.Command{
	Use:     "init",
	Aliases: []string{"scaffold"},
	Short:   "Scaffold a repo — an app repo (platform.toml + build script) or an infra repo (full GitOps baseline)",
	Run:     runInit,
}

func init() {
	InitCmd.Flags().BoolVar(&initForce, "force", false,
		"replace existing files instead of keeping them")
}

func runInit(cmd *cobra.Command, args []string) {
	wd, err := os.Getwd()
	if err != nil {
		buildlog.Fatalln(err)
	}
	sess := prompts.New(nil, args)

	// An infra repo (name matches the infra glob — see builder.IsInfra) gets the embedded
	// GitOps baseline (components, defaults/, cue.mod); any other repo gets just
	// platform.toml + the build script.
	if builder.IsInfra(wd) {
		applyPlan(wd, sess, planInfra(wd, sess))
	} else {
		applyPlan(wd, sess, planApp(wd, sess))
	}
}

// planApp scaffolds an ordinary app repo: platform.toml (with discovered modules) plus the
// executable platform build script.
func planApp(wd string, sess *prompts.Session) *scaffold.Plan {
	info := &scaffold.Info{
		ProjectName:     filepath.Base(wd),
		GoVersion:       runtime.Version()[2:],
		Maintainer:      sess.Str("your name"),
		MaintainerEmail: sess.Str("your email"),
		Repository:      sess.Str("github repository address (without https:// prefix)"),
		ImagePrefix:     sess.Str("docker image prefix (e.g. ghcr.io/prod9/)"),
	}
	plan, err := scaffold.Analyze(wd, info, nil)
	if err != nil {
		buildlog.Fatalln(err)
	}
	return plan
}

// planInfra scaffolds an infra repo: the app-repo base plus the embedded GitOps baseline
// (operator-selected components, the mandatory defaults/ package, cue.mod).
func planInfra(wd string, sess *prompts.Session) *scaffold.Plan {
	daggerVersion := baseline.DaggerVersion()
	if daggerVersion == "" {
		buildlog.Fatalln(errors.New("could not determine the linked dagger SDK version"))
	}

	info := &scaffold.Info{
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
	if scaffold.HasCueModule(wd) {
		var err error
		info.ModulePath, err = scaffold.ModulePath(wd)
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
	plan, err := scaffold.AnalyzeInit(wd, info, vars)
	if err != nil {
		buildlog.Fatalln(err)
	}
	for _, f := range rendered {
		plan.AddFile(f.Path, f.Body, 0644)
	}
	return plan
}

// applyPlan is the shared tail: show the plan, confirm, ensure a git repo, write the files,
// then print the effective parsed config so the operator sees the resolved result in one shot.
func applyPlan(wd string, sess *prompts.Session, plan *scaffold.Plan) {
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

	// Close with the effective parsed config (same view as `configure`) so the operator sees
	// the resolved result of the freshly written platform.toml.
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

// ensureGitRepo runs `git init` when dir is not already its own git repo root — an infra
// scaffold creates a standalone repo (GitOps delivery needs one), even nested inside another
// checkout. For an app repo (which already has git) this is a no-op.
func ensureGitRepo(dir string) error {
	if scaffold.IsGitRoot(dir) {
		return nil
	}
	gitInit := exec.Command("git", "init", dir)
	gitInit.Stdout, gitInit.Stderr = os.Stdout, os.Stderr
	return gitInit.Run()
}
