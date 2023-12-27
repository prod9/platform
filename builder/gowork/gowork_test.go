package gowork

import (
	"github.com/stretchr/testify/require"
	"testing"
)

const GoWorkFixture = `
go 1.21

use (
	./core
	./fx
	./platform
	./x9
)
`

const GoModFixture = `
module platform.prodigy9.co

go 1.21.4

require github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d

require (
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.16 // indirect
	golang.org/x/sys v0.0.0-20220811171246-fbc7d0a398ab // indirect
)
`

func TestParseString(t *testing.T) {
	version, mods, err := ParseString(GoWorkFixture)
	require.NoError(t, err)
	require.Equal(t, "1.21.0", version)
	require.ElementsMatch(t, mods, []string{
		"core",
		"fx",
		"platform",
		"x9",
	})
}

func TestParseString_GoMod(t *testing.T) {
	version, _, err := ParseString(GoModFixture)
	require.NoError(t, err)
	require.Equal(t, "1.21.4", version)
}
