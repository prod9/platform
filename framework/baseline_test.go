package framework

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/mod/modfile"
	r "github.com/stretchr/testify/require"
	"platform.prodigy9.co/dsl"
	"platform.prodigy9.co/framework/scaffold"
	"platform.prodigy9.co/project"
)

// infraSpec runs Infra.Scaffold against a temp dir and indexes its files by path.
func infraSpec(t *testing.T, wd string) (scaffold.Spec, map[string]scaffold.File) {
	t.Helper()

	spec, err := Infra{}.Scaffold(context.Background(), wd)
	r.NoError(t, err)

	byPath := map[string]scaffold.File{}
	for _, f := range spec.Files {
		byPath[f.Path] = f
	}
	return spec, byPath
}

// TestInfraScaffoldContributesBaseline asserts the rich Scaffold output: the whole
// baseline routed by destination, the version-pin vars, the "rolling" strategy seed, and
// the fresh-repo need — with no app-vs-infra predicate anywhere.
func TestInfraScaffoldContributesBaseline(t *testing.T) {
	spec, byPath := infraSpec(t, t.TempDir())

	r.Equal(t, "rolling", spec.Strategy)
	r.True(t, spec.NeedsGitRepo)
	r.NotNil(t, spec.Module)
	r.Equal(t, "platform/infra", spec.Module.Framework)
	r.Contains(t, spec.Vars, "CERT_MANAGER_VERSION")

	// Destination-encoded routing applied; .tmpl holes left unresolved for the driver.
	r.Contains(t, byPath, filepath.Join("apps", "cert-manager.platform"))
	r.Contains(t, byPath, filepath.Join("apps", "platform.cue.tmpl"))
	r.Contains(t, byPath, filepath.Join("defaults", "basics.cue"))
	r.Contains(t, byPath, filepath.Join("cue.mod", "module.cue.tmpl"))
}

// TestInfraScaffoldCueModule resolves the greenfield cue.mod contribution and checks the
// shape `platform render` loads: module path hole resolved, the linked evaluator's
// language version, the defs dep pinned. An existing cue.mod suppresses the contribution.
func TestInfraScaffoldCueModule(t *testing.T) {
	_, byPath := infraSpec(t, t.TempDir())

	files, err := scaffold.Resolve(
		[]scaffold.File{byPath[filepath.Join("cue.mod", "module.cue.tmpl")]},
		scaffold.Data{ModulePath: "test.example/infra"})
	r.NoError(t, err)
	r.Equal(t, filepath.Join("cue.mod", "module.cue"), files[0].Path)

	mf, err := modfile.Parse(files[0].Content, files[0].Path)
	r.NoError(t, err)
	r.Equal(t, "test.example/infra", mf.Module)
	r.Equal(t, cue.LanguageVersion(), mf.Language.Version)
	r.Contains(t, mf.Deps, DefsModule)
	r.Equal(t, DefsVersion, mf.Deps[DefsModule].Version)
}

func TestCueModulePathStripsMajorSuffix(t *testing.T) {
	// Callers form import paths like `<module>/defaults`, so the `@vN` major-version
	// suffix of an existing module must be stripped.
	dir := t.TempDir()
	mod := filepath.Join(dir, "cue.mod", "module.cue")
	r.NoError(t, os.MkdirAll(filepath.Dir(mod), 0o755))
	r.NoError(t, os.WriteFile(mod,
		[]byte("module: \"kept.example/infra@v0\"\nlanguage: version: \"v0.15.4\"\n"), 0o644))

	path, err := CueModulePath(dir)
	r.NoError(t, err)
	r.Equal(t, "kept.example/infra", path)
}

func TestInfraScaffoldKeepsExistingCueModule(t *testing.T) {
	dir := t.TempDir()
	mod := filepath.Join(dir, "cue.mod", "module.cue")
	r.NoError(t, os.MkdirAll(filepath.Dir(mod), 0o755))
	r.NoError(t, os.WriteFile(mod,
		[]byte("module: \"kept.example/infra\"\nlanguage: version: \"v0.15.4\"\n"), 0o644))

	_, byPath := infraSpec(t, dir)
	r.NotContains(t, byPath, filepath.Join("cue.mod", "module.cue.tmpl"),
		"existing cue.mod must not be re-scaffolded")
}

// TestEmbeddedCertManager runs the embedded cert-manager directive through the DSL with a
// fixture fetcher, asserting the version var interpolates into the download URL and the
// fetched manifest is emitted under its filename.
func TestEmbeddedCertManager(t *testing.T) {
	_, byPath := infraSpec(t, t.TempDir())
	body := byPath[filepath.Join("apps", "cert-manager.platform")].Content
	r.NotEmpty(t, body)

	version := fmt.Sprint(DefaultVars["CERT_MANAGER_VERSION"])
	r.NotEmpty(t, version)

	var gotURL string
	fetch := func(url string) ([]byte, error) {
		gotURL = url
		return []byte("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: cert-manager\n"), nil
	}

	out := t.TempDir()
	_, err := dsl.Apply(string(body), dsl.Options{
		Vars:   project.NormalizeVars(DefaultVars),
		OutDir: out,
		Fetch:  fetch,
	})
	r.NoError(t, err)

	r.Contains(t, gotURL, version, "cert_manager_version not interpolated into download URL")

	emitted, err := os.ReadFile(filepath.Join(out, "cert-manager.yaml"))
	r.NoError(t, err)
	r.Contains(t, string(emitted), "kind: Namespace")
}

// TestEmbeddedNginxGateway runs the embedded NGF directive end to end with a fixture
// fetcher: it asserts the version vars interpolate into the three download URLs, and that
// the NginxProxy patch lands serverTokens=off plus the Linode firewall annotation as a
// STRING (the value-typing fix — a bare int there would be invalid).
func TestEmbeddedNginxGateway(t *testing.T) {
	_, byPath := infraSpec(t, t.TempDir())
	body := byPath[filepath.Join("apps", "nginx-gateway-exp.platform")].Content
	r.NotEmpty(t, body)

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
	_, err := dsl.Apply(string(body), dsl.Options{
		Vars:   project.NormalizeVars(DefaultVars),
		OutDir: out,
		Fetch:  fetch,
	})
	r.NoError(t, err)

	joined := strings.Join(urls, "\n")
	for _, want := range []string{
		fmt.Sprint(DefaultVars["GATEWAY_API_VERSION"]),
		fmt.Sprint(DefaultVars["NGINX_GATEWAY_VERSION"]),
	} {
		r.Contains(t, joined, want, "version not interpolated into a download URL")
	}

	ngf, err := os.ReadFile(filepath.Join(out, "nginx-gateway.yaml"))
	r.NoError(t, err)
	got := string(ngf)

	r.Contains(t, got, `serverTokens: "off"`)
	// The firewall id must stay a string — yaml quotes it; a bare int would be invalid.
	r.Contains(t, got, `linode-loadbalancer-firewall-id: "11222746"`)
	r.Contains(t, got, "type: StrategicMerge")
}
