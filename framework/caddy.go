package framework

import (
	"fmt"

	"dagger.io/dagger"
)

const (
	// CaddyfilePath is Caddy's own packaged config path, and deliberately outside RunDir:
	// the served root serves everything in it, a config file included.
	CaddyfilePath = "/etc/caddy/Caddyfile"

	// DefaultHTTPPort is where a static runner listens when the module names no `port`.
	DefaultHTTPPort = 3000

	// hashedAssetPath matches content-addressed bundle output, whose names change with
	// their contents and so can be cached forever. `_astro` is Astro's default, and Astro
	// is what the static family discovers on.
	hashedAssetPath = "/_astro/*"

	// caddyfileTemplate is the static family's whole HTTP surface — headers, cache tiers,
	// compression, error pages and access log, none of which the `caddy file-server`
	// subcommand can express. Spec: docs/spec/frameworks.md, "The static family's HTTP
	// surface"; change one rule here and change it there.
	caddyfileTemplate = `{
	admin off
	auto_https off
	servers {
		trusted_proxies static private_ranges
	}
}

:%d {
	root * %s
	encode zstd gzip

	header X-Content-Type-Options nosniff

	@immutable path %s
	header @immutable Cache-Control "public, max-age=31536000, immutable"

	@revalidate not path %s
	header @revalidate Cache-Control "public, max-age=0, must-revalidate"

	file_server

	handle_errors {
		rewrite * /{err.status_code}.html
		file_server
	}

	log {
		output stdout
		format json
	}
}
`
)

// withCaddyServer installs Caddy and lays down the config it serves under. Both halves
// belong to one call: Caddy without this config is the packaged default, which serves a
// placeholder page from a directory the build never writes.
func withCaddyServer(base *dagger.Container, unit *BuildUnit) *dagger.Container {
	return withPkgs(base, "caddy").
		WithNewFile(CaddyfilePath, caddyfile(unit)).
		WithExposedPort(httpPort(unit))
}

// caddyRunArgs starts Caddy against that config. The `file-server` subcommand is not an
// alternative — it takes no config at all.
func caddyRunArgs() []string {
	return []string{"run", "--config", CaddyfilePath, "--adapter", "caddyfile"}
}

func caddyfile(unit *BuildUnit) string {
	return fmt.Sprintf(caddyfileTemplate,
		httpPort(unit), RunDir, hashedAssetPath, hashedAssetPath)
}

func httpPort(unit *BuildUnit) int {
	if unit.Port > 0 {
		return unit.Port
	}
	return DefaultHTTPPort
}
