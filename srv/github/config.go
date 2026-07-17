package github

import "fx.prodigy9.co/config"

// Server + GitHub endpoint config, shared with srv/auth.
var (
	ServerURLConfig = config.Str("SERVER_URL")
	URLConfig       = config.StrDef("GITHUB_URL", "https://github.com")
	APIURLConfig    = config.StrDef("GITHUB_API_URL", "https://api.github.com")
)

// GitHub App credentials. The App is created by hand on GitHub (guided by the install
// page) and its credentials injected here via fx config — a k8s Secret at rest, read
// directly, never stored in the DB. See docs/spec/platform-server.md §"srv owns the App".
var (
	AppIDConfig         = config.Int64("GITHUB_APP_ID")
	SlugConfig          = config.Str("GITHUB_APP_SLUG")
	PrivateKeyConfig    = config.Str("GITHUB_APP_PRIVATE_KEY")
	WebhookSecretConfig = config.Str("GITHUB_APP_WEBHOOK_SECRET")
	ClientIDConfig      = config.Str("GITHUB_APP_CLIENT_ID")
	ClientSecretConfig  = config.Str("GITHUB_APP_CLIENT_SECRET")
)
