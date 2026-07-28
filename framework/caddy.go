package framework

import (
	"fmt"

	"dagger.io/dagger"
)

// caddyfilePath is Caddy's own packaged config path, and deliberately outside RunDir: the
// served root serves everything in it, a config file included.
const caddyfilePath = "/etc/caddy/Caddyfile"

// caddyfile is the static family's HTTP surface. Response headers, cache policy, error
// pages, trusted proxies and the admin endpoint have no `caddy file-server` flags, which
// is why the subcommand is not an alternative. Still a constant in the sense that matters:
// the only thing interpolated is RunDir, platform's own path, never anything from the
// project being built. Spelling that path out a second time here is how it goes stale the
// day the FHS tree moves.
//
// `{$PORT:3000}` is Caddy's own pre-parse env substitution, not a Go one — Caddy resolves
// it against the container's environment when the runner starts, so the file holds the same
// bytes in every static image and platform never learns the port. Nothing here or anywhere
// else sets PORT; it arrives from whoever runs the image, or it does not and 3000 stands.
// Spec: docs/spec/frameworks.md, "The static family's HTTP surface"; Caddy's own behavior:
// docs/vendor/caddy.md.
var caddyfile = fmt.Sprintf(`{
	admin off
	servers {
		trusted_proxies static private_ranges
	}
}

:{$PORT:3000} {
	root * %s
	encode zstd gzip

	header X-Content-Type-Options nosniff
	header Cache-Control "public, max-age=0, must-revalidate"

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
`, RunDir)

func withCaddyServer(base *dagger.Container) *dagger.Container {
	return withPkgs(base, "caddy").WithNewFile(caddyfilePath, caddyfile)
}
