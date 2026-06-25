// Package baseline ships the embedded cluster-baseline: the component files
// (`.platform` directives + `.cue` apps) platform installs into a fresh infra repo,
// plus the default version pins they interpolate. There is no marker grammar and no
// render-time gating — init offers the whole built-in list with Defaults pre-checked,
// installs whatever the operator keeps, and render applies whatever was installed.
package baseline

import (
	"embed"
	"io/fs"
)

//go:embed files/*.platform files/*.cue
var embedded embed.FS

// DefsModule is the infra-defs CUE dependency the baseline apps import; DefsVersion is the
// version a freshly-init'd infra repo pins into its cue.mod. v0.3.21 is the engine-supporting
// defs — it carries #Service #headless and parts.#PodMounts #claim_templates, both of which
// dagger-engine.cue needs to render.
const (
	DefsModule  = "prodigy9.co/defs@v0"
	DefsVersion = "v0.3.21"
)

// DefaultVars is the baseline's shipped [ops.vars]: the version pins each baseline hook
// consumes. Keys are env-style (SCREAMING_SNAKE) — the preferred platform.toml form; render
// normalizes them to lowercase for both consumption routes, `\(cert_manager_version)` in
// directives and `@tag(cert_manager_version)` in CUE apps. Bootstrap seeds these into a fresh
// platform.toml and merges on re-bootstrap (new keys appended, operator values preserved).
// Pure interpolation inputs — component selection is not a var.
var DefaultVars = map[string]any{
	"CERT_MANAGER_VERSION":  "v1.20.2",
	"FLUX_VERSION":          "v2.8.8",
	"ARGOCD_VERSION":        "v3.4.1",
	"NGINX_GATEWAY_VERSION": "v2.6.0",
	"GATEWAY_API_VERSION":   "v1.5.1",

	"NGINX_GATEWAY_FIREWALL_ID": "11222746", // Linode LB firewall; string, not int
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
