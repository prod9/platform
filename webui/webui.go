// Package webui owns the web UI embedded into the platform binary: the SvelteKit
// source (src/, plain JS, adapter-static) and its committed build output in build/.
// Rebuild with `pnpm build` and commit the result — the binary ships whatever build/
// holds.
package webui

import "embed"

//go:embed all:build
var Assets embed.FS
