// Package webui owns the built web UI assets embedded into the platform binary. The
// SvelteKit source lives alongside; its adapter-static output lands in build/.
package webui

import "embed"

//go:embed all:build
var Assets embed.FS
