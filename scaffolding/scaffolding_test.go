package scaffolding

import (
	"os"
	"path/filepath"
	"testing"

	r "github.com/stretchr/testify/require"
)

func TestDiscover_exposesFrameworkScaffoldVars(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test-infra")
	r.NoError(t, os.Mkdir(dir, 0o755))
	r.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))

	target, err := Discover(dir)
	r.NoError(t, err)

	r.Equal(t, []string{"CUE_MOD_PREFIX"}, target.ScaffoldVars())
}

func TestTargetPlan_keepsDiscoveredFramework(t *testing.T) {
	dir := gitRepo(t)
	r.NoError(t, os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\n"), 0o644))
	stubVersions(t)

	target, err := Discover(dir)
	r.NoError(t, err)

	// Go has higher discovery precedence than Dockerfile. Adding its marker after discovery
	// proves planning uses the retained framework instead of inspecting the target again.
	r.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n\ngo 1.26\n"), 0o644))
	plan, err := target.Plan(testInfo(), nil)
	r.NoError(t, err)

	var platformTOML []byte
	for _, file := range plan.Files {
		if file.Path == "platform.toml" {
			platformTOML = file.Content
		}
	}
	r.Contains(t, string(platformTOML), `framework = "dockerfile"`)
}
