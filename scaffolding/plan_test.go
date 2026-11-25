package scaffolding

import (
	"os"
	"path/filepath"
	"testing"

	r "github.com/stretchr/testify/require"
)

func testInfo() Info {
	return Info{
		Maintainer:      "A",
		MaintainerEmail: "a@b.co",
		Repository:      "github.com/prod9/app",
	}
}

// stubVersions pins the build-info seam: test binaries carry no versions in their build
// info, so every Plan test stubs the platform-release lookup.
func stubVersions(t *testing.T) {
	t.Helper()
	origPlatform := platformVersion
	platformVersion = func() string { return "v0.9.1" }
	t.Cleanup(func() { platformVersion = origPlatform })
}

func TestPlan_freshRepoWritesEverything(t *testing.T) {
	dir := gitRepo(t)
	stubVersions(t)

	target, err := Discover(dir)
	r.NoError(t, err)
	plan, err := target.Plan(testInfo(), nil)
	r.NoError(t, err)

	// Every expected output is a fresh write, never an overwrite.
	byPath := map[string]FileChange{}
	for _, f := range plan.Files {
		byPath[f.Path] = f
		r.Equal(t, FileWrite, f.Action, "%s should be a fresh write", f.Path)
	}
	r.Contains(t, byPath, "platform.toml")
	r.Contains(t, byPath, "platform")

	// The launcher pins the binary's version verbatim — never a stale literal.
	r.Contains(t, string(byPath["platform"].Content), `PLATFORM_VERSION="v0.9.1"`)

	// Apply lands them on disk; the platform script is executable.
	_, err = plan.Apply()
	r.NoError(t, err)
	info, err := os.Stat(filepath.Join(dir, "platform"))
	r.NoError(t, err)
	r.NotZero(t, info.Mode()&0100, "platform script must be executable")
}

func TestPlan_infraGetsItsModuleUniformly(t *testing.T) {
	// A dir the Infra framework discovers (name glob) gets its contribution through the same
	// driver path as every other framework — no infra branch. Like every target, it must
	// already be a git repo root (platform never runs `git init`). Until init fetches the
	// infra-lua rock, that contribution is the module and the strategy seed alone
	// (docs/spec/scaffolding.md).
	dir := filepath.Join(t.TempDir(), "test-infra")
	r.NoError(t, os.Mkdir(dir, 0o755))
	r.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))

	stubVersions(t)

	target, err := Discover(dir)
	r.NoError(t, err)
	plan, err := target.Plan(testInfo(), nil)
	r.NoError(t, err)

	paths := []string{}
	byPath := map[string]FileChange{}
	for _, f := range plan.Files {
		paths = append(paths, f.Path)
		byPath[f.Path] = f
	}
	r.ElementsMatch(t, []string{"platform.toml", "platform"}, paths)

	toml := string(byPath["platform.toml"].Content)
	r.Contains(t, toml, `strategy = "rolling"`)
	r.Contains(t, toml, `framework = "platform/infra"`)
}

func TestDiscover_rejectsNonGitDir(t *testing.T) {
	_, err := Discover(t.TempDir())
	r.ErrorIs(t, err, ErrTargetNotGit)
}

func TestPlan_rescaffoldPreservesExisting(t *testing.T) {
	dir := gitRepo(t)
	stubVersions(t)
	existing := `maintainer = "operator <op@b.co>"
repository = "github.com/prod9/app"

[vars]
cert_manager_version = "v1.16.0"
`
	r.NoError(t, os.WriteFile(filepath.Join(dir, "platform.toml"), []byte(existing), 0644))

	target, err := Discover(dir)
	r.NoError(t, err)
	plan, err := target.Plan(testInfo(), nil)
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
	stubVersions(t)
	existing := `maintainer = "operator <op@b.co>"`
	projectPath := filepath.Join(dir, "platform.toml")
	launcherPath := filepath.Join(dir, "platform")
	r.NoError(t, os.WriteFile(projectPath, []byte(existing), 0o644))
	r.NoError(t, os.WriteFile(launcherPath, []byte("operator launcher\n"), 0o744))

	target, err := Discover(dir)
	r.NoError(t, err)
	plan, err := target.Plan(testInfo(), nil)
	r.NoError(t, err)
	r.Positive(t, plan.Overwrites())

	// replace=false leaves the existing overwrite target untouched while still
	// landing the fresh writes.
	applied, err := plan.Apply()
	r.NoError(t, err)
	r.Len(t, applied, 0)
	got, err := os.ReadFile(projectPath)
	r.NoError(t, err)
	r.Equal(t, existing, string(got))

	got, err = os.ReadFile(launcherPath)
	r.NoError(t, err)
	r.Equal(t, "operator launcher\n", string(got))
}

func TestForceApply_replacesExisting(t *testing.T) {
	dir := gitRepo(t)
	stubVersions(t)
	path := filepath.Join(dir, "platform")
	r.NoError(t, os.WriteFile(path, []byte("old launcher\n"), 0o744))

	target, err := Discover(dir)
	r.NoError(t, err)
	plan, err := target.Plan(testInfo(), nil)
	r.NoError(t, err)
	applied, err := plan.ForceApply()
	r.NoError(t, err)
	r.Len(t, applied, len(plan.Files))

	content, err := os.ReadFile(path)
	r.NoError(t, err)
	r.NotContains(t, string(content), "old launcher")
	r.Contains(t, string(content), `PLATFORM_VERSION="v0.9.1"`)
}
