// Package gitops renders an infra repo's apps/*.lua to one manifest tree, a pure function
// of the committed repo and its [vars] table (docs/spec/lua-render.md).
// The Infra framework packs the result into a plain image published by the ordinary
// publish path.
package gitops

import (
	"errors"
	"net/http"
)

const (
	luaExt = ".lua"
)

var (
	errNoModule = errors.New("module not found")

	// downloadClient fetches for platform.download; tests swap it to trust a TLS fixture.
	downloadClient = http.DefaultClient
)
