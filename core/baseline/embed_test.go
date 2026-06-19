package baseline_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"platform.prodigy9.co/core/baseline"
	"platform.prodigy9.co/core/dsl"
)

// TestEmbeddedCertManager runs the embedded cert-manager directive through the
// DSL with a fixture fetcher, asserting the version var interpolates into the
// download URL and the fetched manifest is emitted under its filename.
func TestEmbeddedCertManager(t *testing.T) {
	files, err := baseline.EmbeddedFiles()
	if err != nil {
		t.Fatalf("EmbeddedFiles: %v", err)
	}

	body, ok := files["cert-manager.platform"]
	if !ok {
		t.Fatalf("cert-manager.platform not embedded; have %v", keys(files))
	}

	version := baseline.DefaultVars["cert_manager_version"]
	if version == "" {
		t.Fatal("DefaultVars missing cert_manager_version")
	}

	var gotURL string
	fetch := func(url string) ([]byte, error) {
		gotURL = url
		return []byte("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: cert-manager\n"), nil
	}

	out := t.TempDir()
	if _, err := dsl.Apply(string(body), dsl.Options{
		Vars:   baseline.DefaultVars,
		OutDir: out,
		Fetch:  fetch,
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if !strings.Contains(gotURL, version) {
		t.Errorf("cert_manager_version not interpolated into download URL: %q", gotURL)
	}

	emitted, err := os.ReadFile(filepath.Join(out, "cert-manager.yaml"))
	if err != nil {
		t.Fatalf("read emitted file: %v", err)
	}
	if !strings.Contains(string(emitted), "kind: Namespace") {
		t.Errorf("emit did not write the downloaded manifest:\n%s", emitted)
	}
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
