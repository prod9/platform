package framework

import (
	"fmt"
	"path/filepath"
	"strings"

	"platform.prodigy9.co/framework/scaffold"
	"platform.prodigy9.co/framework/skel"
)

// The cluster baseline: the component files (`.platform` directives + `.cue` apps,
// shipped via the skel collection) the Infra framework scaffolds into a fresh infra
// repo, plus the default version pins they interpolate. There is no marker grammar, no
// render-time gating, and no init-time picker — Infra.Scaffold contributes the fixed
// components set unconditionally and `render` applies whatever was installed.

// DefsModule is the infra-defs CUE dependency the baseline apps import; DefsVersion is the
// version a freshly-init'd infra repo pins into its cue.mod. v0.4.0 carries the #NetworkPolicy
// access-grant pattern and #pod_labels that platform.cue needs to lock the engine's TCP port to
// the dispatcher (atop the v0.3.x #Service #headless + parts.#PodMounts #claim_templates the
// engine StatefulSet renders with).
const (
	DefsModule  = "prodigy9.co/defs@v0"
	DefsVersion = "v0.4.0"
)

// DefaultVars is the baseline's shipped [vars]: the version pins each baseline hook
// consumes. Keys are env-style (SCREAMING_SNAKE) — the preferred platform.toml form; render
// normalizes them to lowercase for both consumption routes, `\(cert_manager_version)` in
// directives and `@tag(cert_manager_version)` in CUE apps. Scaffold seeds these into a fresh
// platform.toml and merges on re-scaffold (new keys appended, operator values preserved).
// Pure interpolation inputs — component selection is not a var.
var DefaultVars = map[string]any{
	"CERT_MANAGER_VERSION":  "v1.20.2",
	"FLUX_VERSION":          "v2.8.8",
	"NGINX_GATEWAY_VERSION": "v2.6.7",
	"GATEWAY_API_VERSION":   "v1.5.1",

	// Per-deployment ingress hosts (render-time @tag holes): the platform server's own vanity
	// host and the Flux webhook-receiver route. prod9 self-host defaults; operators edit.
	"PLATFORM_HOSTNAME": "platform.prodigy9.co",
	"FLUX_HOSTNAME":     "flux.prodigy9.co",
}

// infrabaseComponents is the fixed working set every fresh infra repo installs — the
// components a functioning cluster needs out of the box plus the shared defaults/ package
// every app imports for #Basics (namespace + registry pull secret). The gateway-api
// channel installed is STANDARD (it serves everything the baseline renders, ListenerSet
// included); the experimental variant (apps-nginx-gateway-exp.platform, + TCPRoute/
// UDPRoute) ships in the binary but is not installed — repos needing it swap by hand.
// apps-platform.cue.tmpl carries the build engine + (for prod9's self-host) the vanity
// server and its NetworkPolicies.
var infrabaseComponents = []string{
	"apps-cert-manager.platform",
	"apps-cluster-issuer.cue.tmpl",
	"apps-flux.platform",
	"apps-flux-sync.cue.tmpl",
	"apps-gateway.cue.tmpl",
	"apps-platform.cue.tmpl",
	"apps-nginx-gateway.platform",
	"defaults-basics.cue",
	"defaults-webapp.cue",
}

// infrabaseFiles returns the baseline as routed, unresolved scaffold files: each component
// pulled from the embed and routed to the destination its name encodes. `.tmpl` holes stay
// unresolved — the driver's Resolve fills them.
func infrabaseFiles() ([]scaffold.File, error) {
	out := make([]scaffold.File, 0, len(infrabaseComponents))
	for _, name := range infrabaseComponents {
		content, err := skel.Read(name)
		if err != nil {
			return nil, fmt.Errorf("baseline component %q is not embedded: %w", name, err)
		}
		out = append(out, scaffold.File{Path: infrabaseDest(name), Content: content, Mode: 0644})
	}
	return out, nil
}

// infrabaseDest maps a baseline filename to its repo-relative destination: `apps-*` →
// `apps/`, `defaults-*` → `defaults/`, anything else → the repo root. The `.tmpl` suffix
// survives — it marks the file for the resolve mechanism, which strips it.
func infrabaseDest(name string) string {
	switch {
	case strings.HasPrefix(name, "apps-"):
		return filepath.Join("apps", strings.TrimPrefix(name, "apps-"))
	case strings.HasPrefix(name, "defaults-"):
		return filepath.Join("defaults", strings.TrimPrefix(name, "defaults-"))
	default:
		return name
	}
}
