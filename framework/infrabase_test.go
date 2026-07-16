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
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/framework/scaffold"
	"platform.prodigy9.co/gitops/dsl"
)

// infraSpec runs Infra.Scaffold against a temp dir (greenfield: CUE_MOD_PREFIX supplied) and
// indexes its resolved files by path.
func infraSpec(t *testing.T, wd string) (scaffold.Spec, map[string]scaffold.File) {
	t.Helper()

	spec, err := Infra{}.Scaffold(context.Background(), wd, "github.com/prod9/infra", "v0.21.7",
		map[string]string{"CUE_MOD_PREFIX": "example.com"})
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
	r.NotNil(t, spec.Module)
	r.Equal(t, "platform/infra", spec.Module.Framework)
	r.Contains(t, spec.Vars, "CERT_MANAGER_VERSION")

	// Destination-encoded routing applied; .tmpl holes resolved by Scaffold (suffix stripped).
	r.Contains(t, byPath, filepath.Join("apps", "cert-manager.platform"))
	r.Contains(t, byPath, filepath.Join("apps", "platform.cue"))
	r.Contains(t, byPath, filepath.Join("defaults", "basics.cue"))
	r.Contains(t, byPath, filepath.Join("cue.mod", "module.cue"))
}

// TestInfraScaffoldCueModule checks the greenfield cue.mod Scaffold resolves: the module path
// hole filled from the CUE_MOD_PREFIX input, the linked evaluator's language version, the defs
// dep pinned. An existing cue.mod suppresses the contribution (TestInfraScaffoldKeepsExisting…).
func TestInfraScaffoldCueModule(t *testing.T) {
	_, byPath := infraSpec(t, t.TempDir())

	mod := byPath[filepath.Join("cue.mod", "module.cue")]
	mf, err := modfile.Parse(mod.Content, mod.Path)
	r.NoError(t, err)
	r.Equal(t, "example.com", mf.Module)
	r.Equal(t, cue.LanguageVersion(), mf.Language.Version)
	r.Contains(t, mf.Deps, DefsModule)
	r.Equal(t, DefsVersion, mf.Deps[DefsModule].Version)
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

func TestInfraRequiredScaffoldInputs(t *testing.T) {
	// Greenfield: the CUE module path is a required operator input.
	r.Equal(t, []string{"CUE_MOD_PREFIX"}, Infra{}.RequiredScaffoldInputs(t.TempDir()))

	// With an existing cue.mod, the path is read from it, never re-asked.
	existing := t.TempDir()
	writeModuleFile(t, existing, "kept.example/infra")
	r.Nil(t, Infra{}.RequiredScaffoldInputs(existing))
}

func TestInfraScaffoldData(t *testing.T) {
	// Greenfield: module path comes from the CUE_MOD_PREFIX input; env facts pass through.
	green := t.TempDir()
	data, err := Infra{}.scaffoldData(green, "github.com/prod9/infra", "v0.21.7",
		map[string]string{"CUE_MOD_PREFIX": "prodigy9.co"})
	r.NoError(t, err)
	r.Equal(t, "prodigy9.co", data.ModulePath)
	r.Equal(t, "v0.21.7", data.DaggerVersion)

	// Infra needs the linked SDK version for the engine image ref — an empty one is a hard
	// error here, not a tagless ref downstream.
	_, err = Infra{}.scaffoldData(green, "r", "", map[string]string{"CUE_MOD_PREFIX": "x.co"})
	r.Error(t, err)

	// An input CUE would reject as a module path (no dot in the first segment) fails fast —
	// this is the exact case a bare GitHub org/repo produces.
	_, err = Infra{}.scaffoldData(green, "r", "v", map[string]string{"CUE_MOD_PREFIX": "prod9/infra-new"})
	r.Error(t, err)

	// An existing cue.mod wins over any input — operator truth.
	existing := t.TempDir()
	writeModuleFile(t, existing, "kept.example/infra")
	data, err = Infra{}.scaffoldData(existing, "r", "v", map[string]string{"CUE_MOD_PREFIX": "ignored.co"})
	r.NoError(t, err)
	r.Equal(t, "kept.example/infra", data.ModulePath)
}

func writeModuleFile(t *testing.T, dir, module string) {
	t.Helper()
	mod := filepath.Join(dir, "cue.mod", "module.cue")
	r.NoError(t, os.MkdirAll(filepath.Dir(mod), 0o755))
	r.NoError(t, os.WriteFile(mod,
		[]byte("module: \""+module+"\"\nlanguage: version: \"v0.15.4\"\n"), 0o644))
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
		Vars:   conf.NormalizeVars(DefaultVars),
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
		Vars:   conf.NormalizeVars(DefaultVars),
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
	// The reserved-IP slot must reach the generated LB Service at creation — the CCM reads
	// it only then (not retrofittable; fix = delete/recreate the Gateway). Empty default:
	// operators set it (or delete the directive) before the Gateway first applies.
	r.Contains(t, got, `linode-loadbalancer-reserved-ipv4: ""`)
	r.Contains(t, got, "type: StrategicMerge")
}

// TestEmbeddedFluxReceiver locks push-driven delivery into the baseline: the scaffolded
// flux-sync app must ship a github-type Flux Receiver (the near-instant reconcile trigger
// the OCIRepository interval only backstops) and the HTTPRoute exposing it. A relapse to
// poll-only delivery trips here. Content-level, not a render assertion — the app imports
// the defs module, which a hermetic unit test cannot resolve.
func TestEmbeddedFluxReceiver(t *testing.T) {
	_, byPath := infraSpec(t, t.TempDir())
	body := string(byPath[filepath.Join("apps", "flux-sync.cue")].Content)
	r.NotEmpty(t, body)

	for _, want := range []string{
		`"Receiver"`,          // notification-controller CR kind
		`"github"`,            // GitHub webhook type
		`"registry_package"`,  // GHCR publish event (X-GitHub-Event header)
		"flux-webhook-token",  // HMAC secret the Receiver validates against
		"defs.#HTTPRoute",     // external exposure of the webhook-receiver service
		"@tag(flux_hostname)", // receiver route host — a render-time var
	} {
		r.Contains(t, body, want, "flux-sync baseline lost its webhook delivery wiring")
	}
}
