package gitops_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"platform.prodigy9.co/core/gitops"
)

// writeModule lays down a minimal CUE module rooted at dir: a cue.mod so `cue
// export` resolves a module root, and an apps package carrying appsSrc.
func writeModule(t *testing.T, dir, appsSrc string) {
	t.Helper()

	mod := filepath.Join(dir, "cue.mod", "module.cue")
	if err := os.MkdirAll(filepath.Dir(mod), 0o755); err != nil {
		t.Fatalf("mkdir cue.mod: %v", err)
	}
	const moduleFile = `module: "test.example/infra@v0"
language: version: "v0.16.0"
`
	if err := os.WriteFile(mod, []byte(moduleFile), 0o644); err != nil {
		t.Fatalf("write module.cue: %v", err)
	}

	apps := filepath.Join(dir, "apps", "sample.cue")
	if err := os.MkdirAll(filepath.Dir(apps), 0o755); err != nil {
		t.Fatalf("mkdir apps: %v", err)
	}
	if err := os.WriteFile(apps, []byte(appsSrc), 0o644); err != nil {
		t.Fatalf("write sample.cue: %v", err)
	}
}

// sampleApps exercises every facet of the file-map contract: a single-doc file,
// a multi-doc list file, an int scalar (fidelity), an image @tag injection, and
// a hidden #out that must not surface as an app.
const sampleApps = `package apps

gateway: {
	"namespace.yaml": {apiVersion: "v1", kind: "Namespace", metadata: name: "gw"}
	"routes.yaml": [
		{apiVersion: "v1", kind: "Service", metadata: name: "svc"},
		{apiVersion: "v1", kind: "Endpoints", metadata: name: "ep"},
	]
	#out: name: "gateway"
}

demo: {
	"deploy.yaml": {apiVersion: "apps/v1", kind: "Deployment", spec: {replicas: 3, image: _img}}
}

_img: string @tag(image)
`

func TestRenderFileMap(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, sampleApps)

	tree, err := gitops.Render(dir, "demo:v1")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	wantPaths := []string{"demo/deploy.yaml", "gateway/namespace.yaml", "gateway/routes.yaml"}
	if got := tree.Paths(); !equalStrings(got, wantPaths) {
		t.Fatalf("paths = %v, want %v (hidden #out must be excluded)", got, wantPaths)
	}

	namespace := string(tree["gateway/namespace.yaml"])
	if strings.Contains(namespace, "---") {
		t.Errorf("single-doc file carries a document separator:\n%s", namespace)
	}

	routes := string(tree["gateway/routes.yaml"])
	if !strings.Contains(routes, "---") || !strings.Contains(routes, "Service") || !strings.Contains(routes, "Endpoints") {
		t.Errorf("list file is not a multi-doc stream:\n%s", routes)
	}

	deploy := string(tree["demo/deploy.yaml"])
	if !strings.Contains(deploy, "replicas: 3") {
		t.Errorf("int scalar lost fidelity (want `replicas: 3`):\n%s", deploy)
	}
	if !strings.Contains(deploy, "image: demo:v1") {
		t.Errorf("@tag(image) not injected (want `image: demo:v1`):\n%s", deploy)
	}
}

func TestTreeWriteDir(t *testing.T) {
	src := t.TempDir()
	writeModule(t, src, sampleApps)

	tree, err := gitops.Render(src, "demo:v1")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := t.TempDir()
	if err := tree.WriteDir(out); err != nil {
		t.Fatalf("WriteDir: %v", err)
	}

	for rel, want := range tree {
		got, err := os.ReadFile(filepath.Join(out, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if string(got) != string(want) {
			t.Errorf("%s on disk mismatches tree content", rel)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	a = append([]string{}, a...)
	b = append([]string{}, b...)
	sort.Strings(a)
	sort.Strings(b)
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
