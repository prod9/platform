package auth

import "embed"

// Migrations is this fragment's schema; srv aggregates every fragment's SQL at boot.
//
//go:embed *.sql
var Migrations embed.FS
