package gitops_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/framework/gitops"
)

// upstream is a third-party install as a release asset serves it: empty documents, a null
// field, an empty list and scalars of each type.
const upstream = `---
apiVersion: v1
kind: Namespace
metadata:
  name: cert-manager
  creationTimestamp: null
---
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cert-manager
spec:
  replicas: 1
  paused: false
  template:
    spec:
      containers:
      - name: controller
        args: []
        ports:
        - containerPort: 9402
      - name: sidecar
        args: []
`

func serveUpstream(t *testing.T, body string) string {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cert-manager.yaml" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	gitops.UseDownloadClient(t, server.Client())
	return server.URL
}

// docs/spec/lua-render.md, "Upstream manifests": the stream arrives as a list of
// documents, empty ones skipped and null fields dropped, edited with plain Lua; an empty
// sequence carries the list mark and renders back as [].
func TestRenderDownloadedManifests(t *testing.T) {
	base := serveUpstream(t, upstream)
	repoDir := newFixtureRepo(t)
	writeRepoFile(t, repoDir, "apps/cert-manager.lua", `
local docs = platform.download("`+base+`/cert-manager.yaml")
assert(#docs == 2, "want 2 documents, got " .. #docs)
local args = docs[2].spec.template.spec.containers[1].args
args[#args + 1] = "--enable-gateway-api"
return {["cert-manager.yaml"] = docs}`)

	tree, err := gitops.Render(repoDir, nil)
	require.NoError(t, err)
	require.Equal(t, `apiVersion: v1
kind: Namespace
metadata:
    name: cert-manager
---
apiVersion: apps/v1
kind: Deployment
metadata:
    name: cert-manager
spec:
    paused: false
    replicas: 1
    template:
        spec:
            containers:
                - args:
                    - --enable-gateway-api
                  name: controller
                  ports:
                    - containerPort: 9402
                - args: []
                  name: sidecar
`, string(tree["cert-manager/cert-manager.yaml"]))
}

// docs/spec/lua-render.md, "Upstream manifests": a non-https URL, a status other than
// 200, a body that is not YAML and a null list item each fail the render, naming the URL.
func TestRenderDownloadFailures(t *testing.T) {
	base := serveUpstream(t, upstream)
	cases := map[string]struct {
		url  string
		body string
	}{
		"plain http":     {url: "http" + strings.TrimPrefix(base, "https") + "/cert-manager.yaml"},
		"not found":      {url: base + "/missing.yaml"},
		"not yaml":       {url: base + "/cert-manager.yaml", body: "a: [\n"},
		"null list item": {url: base + "/cert-manager.yaml", body: "kind: List\nitems:\n- ~\n"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			url := c.url
			if c.body != "" {
				url = serveUpstream(t, c.body) + "/cert-manager.yaml"
			}
			repoDir := t.TempDir()
			writeRepoFile(t, repoDir, "apps/web.lua", `return {["a.yaml"] = platform.download("`+url+`")}`)

			_, err := gitops.Render(repoDir, nil)
			require.ErrorContains(t, err, "platform.download")
			require.ErrorContains(t, err, url)
		})
	}
}
