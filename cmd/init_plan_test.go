package cmd

import (
	"os"
	"path/filepath"
	"testing"

	r "github.com/stretchr/testify/require"
)

func testInfo() *Info {
	return &Info{
		Maintainer:      "A",
		MaintainerEmail: "a@b.co",
		Repository:      "github.com/prod9/app",
	}
}

func TestAnalyze_freshRepoWritesEverything(t *testing.T) {
	dir := gitRepo(t)

	plan, err := Analyze(dir, testInfo())
	r.NoError(t, err)

	// Every expected output is a fresh write, never an overwrite.
	byPath := map[string]FileChange{}
	for _, f := range plan.Files {
		byPath[f.Path] = f
		r.Equal(t, FileWrite, f.Action, "%s should be a fresh write", f.Path)
	}
	r.Contains(t, byPath, "platform.toml")
	r.Contains(t, byPath, "platform")

	// Apply lands them on disk; the platform script is executable.
	r.NoError(t, plan.Apply())
	info, err := os.Stat(filepath.Join(dir, "platform"))
	r.NoError(t, err)
	r.NotZero(t, info.Mode()&0100, "platform script must be executable")
}

func TestAnalyze_infraGetsBaselineUniformly(t *testing.T) {
	// A dir the Infra framework discovers (name glob) needs no pre-existing git repo; the
	// plan carries the framework's fresh-repo need and its full baseline contribution —
	// same driver path as every other framework, no infra branch.
	dir := filepath.Join(t.TempDir(), "test-infra")
	r.NoError(t, os.Mkdir(dir, 0o755))

	// Test binaries carry no dep versions in build info; stub the linked-SDK lookup.
	orig := daggerVersion
	daggerVersion = func() string { return "v0.21.7" }
	t.Cleanup(func() { daggerVersion = orig })

	plan, err := Analyze(dir, testInfo())
	r.NoError(t, err)
	r.True(t, plan.NeedsGitRepo)

	byPath := map[string]FileChange{}
	for _, f := range plan.Files {
		byPath[f.Path] = f
	}
	r.Contains(t, byPath, "platform.toml")
	r.Contains(t, byPath, filepath.Join("apps", "cert-manager.platform"))
	r.Contains(t, byPath, filepath.Join("defaults", "basics.cue"))
	r.Contains(t, byPath, filepath.Join("cue.mod", "module.cue"))

	// The strategy and import_prefix seeds land in platform.toml.
	toml := string(byPath["platform.toml"].Content)
	r.Contains(t, toml, `strategy = "rolling"`)
	r.Contains(t, toml, `import_prefix = "example.com"`)

	// The cue.mod module path resolves from import_prefix, NOT the GitHub repository —
	// they are separate namespaces (see planSpecFiles).
	cuemod := string(byPath[filepath.Join("cue.mod", "module.cue")].Content)
	r.Contains(t, cuemod, `module: "example.com"`)
	r.NotContains(t, cuemod, testInfo().Repository)
}

func TestAnalyze_rejectsNonGitDir(t *testing.T) {
	_, err := Analyze(t.TempDir(), testInfo())
	r.ErrorIs(t, err, ErrWDNotGit)
}

func TestAnalyze_rescaffoldPreservesExisting(t *testing.T) {
	dir := gitRepo(t)
	existing := `maintainer = "operator <op@b.co>"
repository = "github.com/prod9/app"

[vars]
cert_manager_version = "v1.16.0"
`
	r.NoError(t, os.WriteFile(filepath.Join(dir, "platform.toml"), []byte(existing), 0644))

	plan, err := Analyze(dir, testInfo())
	r.NoError(t, err)

	// An existing platform.toml is rewritten via surgical merge, never regenerated — with no
	// framework contributing vars the operator content survives byte-for-byte. (The
	// var-append merge itself is covered by project's vars_merge tests.)
	var toml FileChange
	for _, f := range plan.Files {
		if f.Path == "platform.toml" {
			toml = f
		}
	}
	r.Equal(t, FileOverwrite, toml.Action)
	r.Equal(t, existing, string(toml.Content))
}

func TestApply_keepsExistingWhenNotReplacing(t *testing.T) {
	dir := gitRepo(t)
	existing := `maintainer = "operator <op@b.co>"`
	path := filepath.Join(dir, "platform.toml")
	r.NoError(t, os.WriteFile(path, []byte(existing), 0644))

	plan, err := Analyze(dir, testInfo())
	r.NoError(t, err)
	r.Positive(t, plan.Overwrites())

	// replace=false leaves the existing overwrite target untouched while still
	// landing the fresh writes.
	r.NoError(t, plan.Apply())
	got, err := os.ReadFile(path)
	r.NoError(t, err)
	r.Equal(t, existing, string(got))

	_, err = os.Stat(filepath.Join(dir, "platform"))
	r.NoError(t, err)
}
