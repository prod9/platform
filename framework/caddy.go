package framework

import "dagger.io/dagger"

// caddyfilePath is Caddy's own packaged config path, and deliberately outside RunDir: the
// served root serves everything in it, a config file included.
const caddyfilePath = "/etc/caddy/Caddyfile"

// caddyfile is the static family's HTTP surface — headers, compression, error pages and
// access log, none of which the `caddy file-server` subcommand can express. A constant:
// nothing in it is derived from the project being built. Spec: docs/spec/frameworks.md,
// "The static family's HTTP surface".
const caddyfile = `{
	admin off
	servers {
		trusted_proxies static private_ranges
	}
}

:3000 {
	root * /platform/run
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
`

// withCaddyServer installs Caddy and lays down the config it serves under.
func withCaddyServer(base *dagger.Container) *dagger.Container {
	return withPkgs(base, "caddy").WithNewFile(caddyfilePath, caddyfile)
}
