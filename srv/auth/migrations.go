package auth

import "embed"

// Migrations is this fragment's schema, registered through auth's fx app fragment.
//
//go:embed *.sql
var Migrations embed.FS
