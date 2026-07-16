// Package webui owns the built web UI assets embedded into the platform binary. The
// SvelteKit source will land alongside in a later slice, its adapter-static output
// replacing the committed placeholder in build/.
package webui

import "embed"

//go:embed all:build
var Assets embed.FS
