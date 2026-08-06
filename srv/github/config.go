package github

import "fx.prodigy9.co/config"

// GitHub endpoint config, shared with srv/auth. App credentials are not
// config — they live in the github.app_* settings (app.go); the server's own
// public URL is the server.public_url setting (settings.go).
var (
	URLConfig    = config.StrDef("GITHUB_URL", "https://github.com")
	APIURLConfig = config.StrDef("GITHUB_API_URL", "https://api.github.com")
)
