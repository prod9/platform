package github

import "fx.prodigy9.co/config"

// Server + GitHub endpoint config, shared with srv/auth. App credentials are not
// config — they live in the github.app_* settings (app.go).
var (
	ServerURLConfig = config.Str("SERVER_URL")
	URLConfig       = config.StrDef("GITHUB_URL", "https://github.com")
	APIURLConfig    = config.StrDef("GITHUB_API_URL", "https://api.github.com")
)
