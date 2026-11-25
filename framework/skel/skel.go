// Package skel holds the file assets platform ships into scaffolded repos — named after
// /etc/skel. A `.tmpl` suffix marks a file with holes for the scaffold resolve mechanism.
// Readers pick what they need; nothing here decides when a repo gets it.
package skel

import _ "embed"

// Launcher is the version-pinned launcher script every scaffolded repo gets. Per the
// collection convention its .tmpl name marks the hole — the platform version — which the
// init driver resolves before writing.
//
//go:embed platform.tmpl
var Launcher []byte
