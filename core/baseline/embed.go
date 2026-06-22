// Package baseline ships the embedded cluster-baseline: the component files
// (`.platform` directives + `.cue` apps) platform installs into a fresh infra repo,
// plus the default version pins they interpolate. There is no marker grammar and no
// render-time gating — init offers the whole built-in list with Defaults pre-checked,
// installs whatever the operator keeps, and render applies whatever was installed.
package baseline

import (
	"embed"
	"io/fs"
	"strings"
)

//go:embed files/*.platform files/*.cue
var embedded embed.FS

const platformExt = ".platform"

// DefaultVars is the baseline's shipped [ops.vars]: the version pins each directive
// interpolates via \(var) into its download URLs. Bootstrap seeds these into a fresh
// platform.toml and merges them on re-bootstrap (new keys appended, operator values
// preserved). These are pure interpolation inputs — component selection is not a var.
var DefaultVars = map[string]any{
	"cert_manager_version":  "v1.20.2",
	"flux_version":          "v2.8.8",
	"argocd_version":        "v3.4.1",
	"nginx_gateway_version": "v2.6.0",
	"gateway_api_version":   "v1.5.1",

	"nginx_gateway_firewall_id": "11222746", // Linode LB firewall; string, not int
}

// Defaults is the working set installed when the operator makes no other choice — the
// components a functioning cluster needs out of the box. argocd (reference install) and
// the stable nginx-gateway are off by default; the operator opts into them at init.
var Defaults = []string{
	"cert-manager.platform",
	"flux.platform",
	"dagger-engine.cue",
	"nginx-gateway-experimental.platform",
}

// EmbeddedFiles returns every built-in component file (both `.platform` directives and
// `.cue` apps) shipped in the binary, keyed by filename. This is the full list init
// offers; the chosen subset is written into the target repo's apps/.
func EmbeddedFiles() (map[string][]byte, error) {
	entries, err := embedded.ReadDir("files")
	if err != nil {
		return nil, err
	}

	files := map[string][]byte{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		content, err := fs.ReadFile(embedded, "files/"+e.Name())
		if err != nil {
			return nil, err
		}
		files[e.Name()] = content
	}
	return files, nil
}

// Component is the output directory a directive renders into: the filename without its
// .platform extension, so a directive's emitted manifests land under k8s/<component>/.
func Component(file string) string {
	return strings.TrimSuffix(file, platformExt)
}
