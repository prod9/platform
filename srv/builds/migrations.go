package builds

import "embed"

// Migrations is this fragment's schema, registered through builds' fx app fragment.
//
//go:embed *.sql
var Migrations embed.FS
