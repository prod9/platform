// Package skel holds the file assets platform ships into scaffolded repos — named after
// /etc/skel. It owns the collection and the naming convention that encodes each file's
// destination: an `apps-`/`defaults-` prefix names the directory, and a `.tmpl` suffix
// marks a file with holes for the scaffold resolve mechanism. Readers pick what they
// need; nothing here decides when a repo gets it.
package skel

import (
	"embed"
	"io/fs"
	"path/filepath"
	"strings"

	"platform.prodigy9.co/framework/scaffold"
)

// Launcher is the version-pinned launcher script every scaffolded repo gets. Per the
// collection convention its .tmpl name marks the hole — the platform version — which the
// init driver resolves before writing.
//
//go:embed platform.tmpl
var Launcher []byte

// CueMod is the greenfield cue.mod/module.cue a CUE-rooted repo gets: the operator's module
// path, the linked CUE evaluator's language version (so render never demands a newer
// language than it links), and the pinned infra-defs dependency the baseline apps import.
// Each is a hole the scaffold Data fills.
//
//go:embed cuemod.tmpl
var CueMod []byte

//go:embed apps-* defaults-*
var components embed.FS

// Files returns the whole component collection as routed, unresolved scaffold files. The
// embed is the list — there is no second copy to keep in sync. `.tmpl` holes stay
// unresolved; the caller's Resolve fills them.
func Files() ([]scaffold.File, error) {
	entries, err := fs.ReadDir(components, ".")
	if err != nil {
		return nil, err
	}

	out := make([]scaffold.File, 0, len(entries))
	for _, entry := range entries {
		content, err := components.ReadFile(entry.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, scaffold.File{Path: Dest(entry.Name()), Content: content, Mode: 0644})
	}
	return out, nil
}

// Dest maps a component filename to its repo-relative destination: `apps-*` → `apps/`,
// `defaults-*` → `defaults/`, anything else → the repo root. The `.tmpl` suffix survives —
// it marks the file for the resolve mechanism, which strips it.
func Dest(name string) string {
	switch {
	case strings.HasPrefix(name, "apps-"):
		return filepath.Join("apps", strings.TrimPrefix(name, "apps-"))
	case strings.HasPrefix(name, "defaults-"):
		return filepath.Join("defaults", strings.TrimPrefix(name, "defaults-"))
	default:
		return name
	}
}
