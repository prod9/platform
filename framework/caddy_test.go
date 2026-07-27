package framework

import (
	"strings"
	"testing"

	r "github.com/stretchr/testify/require"
)

// TestCaddyfile_listensOnTheConfiguredPort is the guard on the one lever an operator has
// over the static runner's socket. `port` used to reach only `preview`'s tunnel while the
// server stayed nailed to 3000, so any port but the default forwarded to nothing.
func TestCaddyfile_listensOnTheConfiguredPort(t *testing.T) {
	r.Contains(t, caddyfile(&BuildUnit{Port: 8080}), "\n:8080 {\n")
	r.Contains(t, caddyfile(&BuildUnit{}), "\n:3000 {\n")
}

// TestCaddyfile_servesTheRunDir pins the served root to the runtime tree, and the config
// itself outside it — a Caddyfile under the served root is a downloadable Caddyfile.
func TestCaddyfile_servesTheRunDir(t *testing.T) {
	conf := caddyfile(&BuildUnit{})
	r.Contains(t, conf, "root * "+RunDir)
	r.False(t, strings.HasPrefix(CaddyfilePath, RunDir))
}

// TestCaddyfile_coversTheHTTPSurface holds the spec'd surface line by line
// (docs/spec/frameworks.md, "The static family's HTTP surface"). Each of these is
// invisible from the outside until something is served, so the unit test is what catches
// a silent drop.
func TestCaddyfile_coversTheHTTPSurface(t *testing.T) {
	conf := caddyfile(&BuildUnit{})
	for _, want := range []string{
		"encode zstd gzip",
		"header X-Content-Type-Options nosniff",
		"@immutable path " + hashedAssetPath,
		`header @immutable Cache-Control "public, max-age=31536000, immutable"`,
		"@revalidate not path " + hashedAssetPath,
		`header @revalidate Cache-Control "public, max-age=0, must-revalidate"`,
		"handle_errors {",
		"rewrite * /{err.status_code}.html",
		"trusted_proxies static private_ranges",
		"output stdout",
	} {
		r.Contains(t, conf, want)
	}
}
