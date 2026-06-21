package baseline

import (
	"embed"
	"io/fs"
	"strings"
)

//go:embed files/*.platform files/apps/*.cue
var embedded embed.FS

const appExt = ".cue"

// DefaultVars is the baseline's shipped [ops.vars]: platform's opinionated
// defaults (version pins and knobs) for the embedded directive set. Bootstrap
// seeds these into a fresh platform.toml and merges them on re-bootstrap (new
// keys appended, operator values preserved). Each key is consumed by a directive
// file via \(var) interpolation.
var DefaultVars = map[string]any{
	"cert_manager_version": "v1.20.2",
	"flux_version":         "v2.8.8",
	"argocd_version":       "v3.4.1",
	"argocd":               "false", // reference install; off by default (toggle)

	"ngf_experimental":          "true", // NGINX Gateway Fabric; on by default (toggle)
	"nginx_gateway_version":     "v2.6.0",
	"gateway_api_version":       "v1.5.1",
	"nginx_gateway_firewall_id": "11222746", // Linode LB firewall; string, not int
}

// EmbeddedFiles returns the baseline directive files (`*.platform`) shipped in the
// binary, keyed by filename. Bootstrap writes these into a target infra repo's
// baseline/.
func EmbeddedFiles() (map[string][]byte, error) {
	return readEmbedded("files", platformExt)
}

// EmbeddedApps returns the CUE app files (`apps/*.cue`) shipped in the binary,
// keyed by filename. Bootstrap writes these into a target infra repo's apps/, the
// CUE-authored half of the baseline (e.g. the Dagger engine StatefulSet).
func EmbeddedApps() (map[string][]byte, error) {
	return readEmbedded("files/apps", appExt)
}

// readEmbedded reads the files with the given extension directly under dir,
// skipping subdirectories. Keys are bare filenames.
func readEmbedded(dir, ext string) (map[string][]byte, error) {
	entries, err := embedded.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	files := map[string][]byte{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ext) {
			continue
		}
		content, err := fs.ReadFile(embedded, dir+"/"+e.Name())
		if err != nil {
			return nil, err
		}
		files[e.Name()] = content
	}
	return files, nil
}
