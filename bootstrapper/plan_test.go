package bootstrapper

import (
	"os"
	"path/filepath"
	"testing"

	r "github.com/stretchr/testify/require"
)

func testInfo() *Info {
	return &Info{
		ProjectName:     "infra",
		Maintainer:      "A",
		MaintainerEmail: "a@b.co",
		Repository:      "github.com/prod9/infra",
		ImagePrefix:     "ghcr.io/prod9/",
		GoVersion:       "1.25.5",
	}
}

func TestAnalyze_freshRepoWritesEverything(t *testing.T) {
	dir := gitRepo(t)

	plan, err := Analyze(dir, testInfo(), nil)
	r.NoError(t, err)

	// Every expected output is a fresh write, never an overwrite.
	byPath := map[string]FileChange{}
	for _, f := range plan.Files {
		byPath[f.Path] = f
		r.Equal(t, FileWrite, f.Action, "%s should be a fresh write", f.Path)
	}
	r.Contains(t, byPath, "platform.toml")
	r.Contains(t, byPath, "platform")
	r.Contains(t, byPath, filepath.Join(".buildkite", "pipeline.yaml"))

	// Apply lands them on disk; the platform script is executable.
	r.NoError(t, plan.Apply())
	info, err := os.Stat(filepath.Join(dir, "platform"))
	r.NoError(t, err)
	r.NotZero(t, info.Mode()&0100, "platform script must be executable")
}

func TestAnalyze_rejectsNonGitDir(t *testing.T) {
	_, err := Analyze(t.TempDir(), testInfo(), nil)
	r.ErrorIs(t, err, ErrWDNotGit)
}

func TestAnalyze_rebootstrapMergesVarsNotClobber(t *testing.T) {
	dir := gitRepo(t)
	existing := `maintainer = "operator <op@b.co>"
repository = "github.com/prod9/infra"

[ops.vars]
cert_manager_version = "v1.16.0"
`
	r.NoError(t, os.WriteFile(filepath.Join(dir, "platform.toml"), []byte(existing), 0644))

	defaults := map[string]any{
		"cert_manager_version": "v1.15.0",
		"flux_version":         "v2.3.0",
	}
	plan, err := Analyze(dir, testInfo(), defaults)
	r.NoError(t, err)

	// An existing platform.toml is overwritten via surgical merge, not a
	// wholesale regenerate — operator sections must survive.
	var toml FileChange
	for _, f := range plan.Files {
		if f.Path == "platform.toml" {
			toml = f
		}
	}
	r.Equal(t, FileOverwrite, toml.Action)

	r.NoError(t, plan.Apply())
	got, err := os.ReadFile(filepath.Join(dir, "platform.toml"))
	r.NoError(t, err)
	out := string(got)
	r.Contains(t, out, `maintainer = "operator <op@b.co>"`) // operator field kept
	r.Contains(t, out, `cert_manager_version = "v1.16.0"`)  // operator var value kept
	r.Contains(t, out, `flux_version = "v2.3.0"`)           // new default appended
}
