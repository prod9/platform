package repos

import "embed"

// Migrations is this fragment's schema, registered through repos' fx app fragment.
//
//go:embed *.sql
var Migrations embed.FS
