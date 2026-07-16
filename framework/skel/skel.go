// Package skel holds the file assets platform ships into scaffolded repos — named after
// /etc/skel. It is storage only: ownership lives with the readers (the init driver writes
// the launcher into every repo; the Infra framework picks its baseline component set).
// A `.tmpl` suffix marks a file with holes for the scaffold resolve mechanism; everything
// else is written verbatim.
package skel

import "embed"

// Launcher is the version-pinned launcher script every scaffolded repo gets. Per the
// collection convention its .tmpl name marks the hole — the platform version — which the
// init driver resolves before writing.
//
//go:embed platform.tmpl
var Launcher []byte

//go:embed apps-* defaults-*
var components embed.FS

// Read returns a shipped component's bytes by name.
func Read(name string) ([]byte, error) {
	return components.ReadFile(name)
}
