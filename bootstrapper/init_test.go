package bootstrapper

import (
	"os"
	"path/filepath"
	"testing"

	r "github.com/stretchr/testify/require"
)

func TestAnalyzeInit_writesProjectFileAndScript(t *testing.T) {
	dir := t.TempDir() // not a git repo — init tolerates that (the cmd git-inits)
	defaults := map[string]any{"cert_manager_version": "v1.20.2", "flux_version": "v2.8.8"}

	plan, err := AnalyzeInit(dir, testInfo(), defaults)
	r.NoError(t, err)

	byPath := map[string]FileChange{}
	for _, f := range plan.Files {
		byPath[f.Path] = f
	}
	r.Contains(t, byPath, "platform.toml")
	// init writes the version-pinned platform launcher script, executable.
	r.Contains(t, byPath, "platform")
	r.Equal(t, os.FileMode(0744), byPath["platform"].Mode)

	r.NoError(t, plan.Apply())
	toml, err := os.ReadFile(filepath.Join(dir, "platform.toml"))
	r.NoError(t, err)
	r.Contains(t, string(toml), "cert_manager_version")
}

// AddFile is how ops-init folds a baseline.Render component into the plan; it must land on
// disk under its routed path when the plan applies.
func TestPlanAddFile_foldsComponentIntoPlan(t *testing.T) {
	dir := t.TempDir()

	plan, err := AnalyzeInit(dir, testInfo(), map[string]any{"cert_manager_version": "v1.20.2"})
	r.NoError(t, err)
	plan.AddFile(filepath.Join("apps", "cert-manager.platform"), []byte("emit \"x.yaml\"\n"), 0644)

	r.NoError(t, plan.Apply())
	got, err := os.ReadFile(filepath.Join(dir, "apps", "cert-manager.platform"))
	r.NoError(t, err)
	r.Contains(t, string(got), `emit "x.yaml"`)
}

func TestAnalyzeInit_rejectsMissingDir(t *testing.T) {
	_, err := AnalyzeInit(filepath.Join(t.TempDir(), "absent"), testInfo(), nil)
	r.Error(t, err)
}
