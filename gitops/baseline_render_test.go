package gitops_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"platform.prodigy9.co/gitops"
)

// writeBaseline lays down .platform directive files under apps/, co-located with the
// CUE apps. Each value is the directive-file body.
func writeBaseline(t *testing.T, dir string, files map[string]string) {
	t.Helper()

	base := filepath.Join(dir, "apps")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir apps: %v", err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(base, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

// fixtureFetch stands in for the network: every download resolves to the same
// minimal manifest, so a test asserts on routing/assembly, not on HTTP.
func fixtureFetch(string) ([]byte, error) {
	return []byte("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: from-fixture\n"), nil
}

// TestRenderBaselineRoute drives the .platform route: every directive present under apps/
// is applied (selection happened at install time — no render-time gating), each emitting
// under its component dir (the filename stem).
func TestRenderBaselineRoute(t *testing.T) {
	dir := t.TempDir()
	writeBaseline(t, dir, map[string]string{
		"nginx-gateway.platform":              "download \"https://fixture/x.yaml\"\nemit \"stable.yaml\"\n",
		"nginx-gateway-experimental.platform": "download \"https://fixture/x.yaml\"\nemit \"experimental.yaml\"\n",
	})

	tree, err := gitops.Render(dir, gitops.RenderOptions{Fetch: fixtureFetch})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// both present → both render; the component dir is the filename stem.
	for _, want := range []string{
		"nginx-gateway/stable.yaml",
		"nginx-gateway-experimental/experimental.yaml",
	} {
		if _, ok := tree[want]; !ok {
			t.Errorf("directive not rendered: %q; paths = %v", want, tree.Paths())
		}
	}
	if got := string(tree["nginx-gateway/stable.yaml"]); !strings.Contains(got, "from-fixture") {
		t.Errorf("emitted file lost the downloaded content:\n%s", got)
	}
}

// TestRenderMergesCueAndBaseline proves one Render call fuses both routes into a
// single tree: the CUE apps package and the .platform baseline coexist.
func TestRenderMergesCueAndBaseline(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, sampleApps)
	writeBaseline(t, dir, map[string]string{
		"cert-manager.platform": "download \"https://fixture/x.yaml\"\nemit \"cert-manager.yaml\"\n",
	})

	tree, err := gitops.Render(dir, gitops.RenderOptions{Vars: map[string]any{"APP_VERSION": "9.9.9"}, Fetch: fixtureFetch})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	for _, want := range []string{"demo/deploy.yaml", "gateway/namespace.yaml", "cert-manager/cert-manager.yaml"} {
		if _, ok := tree[want]; !ok {
			t.Errorf("merged tree missing %q; paths = %v", want, tree.Paths())
		}
	}
}
