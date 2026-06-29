package bootstrapper

import (
	"os"
	"path/filepath"
	"testing"

	r "github.com/stretchr/testify/require"
)

func TestAnalyzeInit_writesComponentsProjectFileAndScript(t *testing.T) {
	dir := t.TempDir() // not a git repo — init tolerates that (the cmd git-inits)

	components := map[string][]byte{
		"cert-manager.platform": []byte("download \"u\"\nemit \"cert-manager.yaml\"\n"),
		"flux.platform":         []byte("download \"u\"\nemit \"flux.yaml\"\n"),
		"dagger-engine.cue":     []byte("package apps\n"),
	}
	defaults := map[string]any{"cert_manager_version": "v1.20.2", "flux_version": "v2.8.8"}

	plan, err := AnalyzeInit(dir, testInfo(), components, defaults)
	r.NoError(t, err)

	byPath := map[string]FileChange{}
	for _, f := range plan.Files {
		byPath[f.Path] = f
	}
	r.Contains(t, byPath, "platform.toml")
	// every selected component — directives and CUE apps alike — lands under apps/.
	r.Contains(t, byPath, filepath.Join("apps", "cert-manager.platform"))
	r.Contains(t, byPath, filepath.Join("apps", "flux.platform"))
	r.Contains(t, byPath, filepath.Join("apps", "dagger-engine.cue"))

	// init writes the version-pinned platform launcher script, executable.
	r.Contains(t, byPath, "platform")
	r.Equal(t, os.FileMode(0744), byPath["platform"].Mode)

	r.NoError(t, plan.Apply())
	got, err := os.ReadFile(filepath.Join(dir, "apps", "cert-manager.platform"))
	r.NoError(t, err)
	r.Contains(t, string(got), `emit "cert-manager.yaml"`)

	app, err := os.ReadFile(filepath.Join(dir, "apps", "dagger-engine.cue"))
	r.NoError(t, err)
	r.Contains(t, string(app), "package apps")

	toml, err := os.ReadFile(filepath.Join(dir, "platform.toml"))
	r.NoError(t, err)
	r.Contains(t, string(toml), "cert_manager_version")
}

func TestAnalyzeInit_rejectsMissingDir(t *testing.T) {
	_, err := AnalyzeInit(filepath.Join(t.TempDir(), "absent"), testInfo(), nil, nil)
	r.Error(t, err)
}
