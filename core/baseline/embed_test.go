package baseline_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

	version := fmt.Sprint(baseline.DefaultVars["cert_manager_version"])
	if version == "" || version == "<nil>" {
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

// TestEmbeddedSelectGating checks the shipped baseline gates correctly: the
// always-on installs render unconditionally, while the argocd toggle is excluded
// by default and included only when its [ops.vars] flag is set.
func TestEmbeddedSelectGating(t *testing.T) {
	files, err := baseline.EmbeddedFiles()
	if err != nil {
		t.Fatalf("EmbeddedFiles: %v", err)
	}
	names := keys(files)

	off, err := baseline.Select(names, baseline.DefaultVars)
	if err != nil {
		t.Fatalf("Select (default): %v", err)
	}
	if slices.Contains(off, "argocd+argocd.platform") {
		t.Errorf("argocd toggle rendered while off by default: %v", off)
	}
	for _, always := range []string{"cert-manager.platform", "flux.platform"} {
		if !slices.Contains(off, always) {
			t.Errorf("always-on %q missing from selection: %v", always, off)
		}
	}

	on, err := baseline.Select(names, map[string]any{"argocd": "true"})
	if err != nil {
		t.Fatalf("Select (argocd on): %v", err)
	}
	if !slices.Contains(on, "argocd+argocd.platform") {
		t.Errorf("argocd toggle not rendered when enabled: %v", on)
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
	body, ok := files["nginx-gateway+ngf_experimental.platform"]
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
		Vars:   baseline.DefaultVars,
		OutDir: out,
		Fetch:  fetch,
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	joined := strings.Join(urls, "\n")
	for _, want := range []string{
		fmt.Sprint(baseline.DefaultVars["gateway_api_version"]),
		fmt.Sprint(baseline.DefaultVars["nginx_gateway_version"]),
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

// TestEmbeddedApps checks the CUE-authored half of the baseline ships separately
// from the .platform directives, keyed by bare filename for init to write under apps/.
func TestEmbeddedApps(t *testing.T) {
	apps, err := baseline.EmbeddedApps()
	if err != nil {
		t.Fatalf("EmbeddedApps: %v", err)
	}
	if _, ok := apps["dagger-engine.cue"]; !ok {
		t.Fatalf("dagger-engine.cue not embedded; have %v", keys(apps))
	}

	// the .platform directives must not leak into the app set.
	if _, ok := apps["cert-manager.platform"]; ok {
		t.Errorf("EmbeddedApps leaked a .platform directive: %v", keys(apps))
	}
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
