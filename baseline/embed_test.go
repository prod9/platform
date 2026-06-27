package baseline_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"platform.prodigy9.co/baseline"
	"platform.prodigy9.co/dsl"
	"platform.prodigy9.co/project"
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

	version := fmt.Sprint(baseline.DefaultVars["CERT_MANAGER_VERSION"])
	if version == "" || version == "<nil>" {
		t.Fatal("DefaultVars missing CERT_MANAGER_VERSION")
	}

	var gotURL string
	fetch := func(url string) ([]byte, error) {
		gotURL = url
		return []byte("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: cert-manager\n"), nil
	}

	out := t.TempDir()
	if _, err := dsl.Apply(string(body), dsl.Options{
		Vars:   project.NormalizeVars(baseline.DefaultVars),
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

// TestDefaultsAreEmbedded checks every shipped default names a real built-in file, and
// the CUE engine app is in the list (no marker grammar, no gating — just a flat set).
func TestDefaultsAreEmbedded(t *testing.T) {
	files, err := baseline.EmbeddedFiles()
	if err != nil {
		t.Fatalf("EmbeddedFiles: %v", err)
	}

	for _, name := range baseline.Defaults {
		if _, ok := files[name]; !ok {
			t.Errorf("Defaults names %q but it is not embedded; have %v", name, keys(files))
		}
	}
	if _, ok := files["platform.cue"]; !ok {
		t.Errorf("platform.cue not embedded; have %v", keys(files))
	}
}

// TestEmbeddedNginxGateway runs the embedded NGF directive end to end with a
// fixture fetcher: it asserts the version vars interpolate into the three
// download URLs, and that the NginxProxy patch lands serverTokens=off plus the
// Linode firewall annotation as a STRING (the value-typing fix — a bare int
// there would be invalid).
func TestEmbeddedNginxGateway(t *testing.T) {
	files, err := baseline.EmbeddedFiles()
	if err != nil {
		t.Fatalf("EmbeddedFiles: %v", err)
	}
	body, ok := files["nginx-gateway-experimental.platform"]
	if !ok {
		t.Fatalf("nginx-gateway directive not embedded; have %v", keys(files))
	}

	var urls []string
	fetch := func(url string) ([]byte, error) {
		urls = append(urls, url)
		switch {
		case strings.Contains(url, "experimental-install"):
			return []byte("kind: CustomResourceDefinition\nmetadata:\n  name: gw\n"), nil
		case strings.Contains(url, "deploy/crds.yaml"):
			return []byte("kind: CustomResourceDefinition\nmetadata:\n  name: ngf\n"), nil
		case strings.Contains(url, "deploy/default/deploy.yaml"):
			return []byte("apiVersion: gateway.nginx.org/v1alpha2\nkind: NginxProxy\n" +
				"metadata:\n  name: ngf-proxy\nspec:\n  ipFamily: dual\n"), nil
		}
		return nil, fmt.Errorf("unexpected url %s", url)
	}

	out := t.TempDir()
	if _, err := dsl.Apply(string(body), dsl.Options{
		Vars:   project.NormalizeVars(baseline.DefaultVars),
		OutDir: out,
		Fetch:  fetch,
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	joined := strings.Join(urls, "\n")
	for _, want := range []string{
		fmt.Sprint(baseline.DefaultVars["GATEWAY_API_VERSION"]),
		fmt.Sprint(baseline.DefaultVars["NGINX_GATEWAY_VERSION"]),
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("version %q not interpolated into a download URL:\n%s", want, joined)
		}
	}

	ngf, err := os.ReadFile(filepath.Join(out, "nginx-gateway.yaml"))
	if err != nil {
		t.Fatalf("read emitted NGF manifest: %v", err)
	}
	got := string(ngf)

	if !strings.Contains(got, `serverTokens: "off"`) {
		t.Errorf("serverTokens=off not patched:\n%s", got)
	}
	// The firewall id must stay a string — yaml quotes it; a bare int would be invalid.
	if !strings.Contains(got, `linode-loadbalancer-firewall-id: "11222746"`) {
		t.Errorf("firewall annotation missing or not a string:\n%s", got)
	}
	if !strings.Contains(got, "type: StrategicMerge") {
		t.Errorf("StrategicMerge patch not built:\n%s", got)
	}
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
