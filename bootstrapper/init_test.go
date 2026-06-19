package bootstrapper

import (
	"os"
	"path/filepath"
	"testing"

	r "github.com/stretchr/testify/require"
)

func TestAnalyzeInit_writesBaselineAndProjectFileOnly(t *testing.T) {
	dir := t.TempDir() // not a git repo — init tolerates that (the cmd git-inits)

	baselineFiles := map[string][]byte{
		"cert-manager.platform": []byte("download \"u\"\nemit \"cert-manager.yaml\"\n"),
		"flux.platform":         []byte("download \"u\"\nemit \"flux.yaml\"\n"),
	}
	defaults := map[string]string{"cert_manager_version": "v1.20.2", "flux_version": "v2.8.8"}

	plan, err := AnalyzeInit(dir, testInfo(), baselineFiles, defaults)
	r.NoError(t, err)

	byPath := map[string]FileChange{}
	for _, f := range plan.Files {
		byPath[f.Path] = f
	}
	r.Contains(t, byPath, "platform.toml")
	r.Contains(t, byPath, filepath.Join("baseline", "cert-manager.platform"))
	r.Contains(t, byPath, filepath.Join("baseline", "flux.platform"))

	// init writes neither the app build script nor the CI pipeline.
	r.NotContains(t, byPath, "platform")
	r.NotContains(t, byPath, filepath.Join(".buildkite", "pipeline.yaml"))

	r.NoError(t, plan.Apply())
	got, err := os.ReadFile(filepath.Join(dir, "baseline", "cert-manager.platform"))
	r.NoError(t, err)
	r.Contains(t, string(got), `emit "cert-manager.yaml"`)

	toml, err := os.ReadFile(filepath.Join(dir, "platform.toml"))
	r.NoError(t, err)
	r.Contains(t, string(toml), "cert_manager_version")
}

func TestAnalyzeInit_rejectsMissingDir(t *testing.T) {
	_, err := AnalyzeInit(filepath.Join(t.TempDir(), "absent"), testInfo(), nil, nil)
	r.Error(t, err)
}
