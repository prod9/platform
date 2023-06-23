package gowork

import (
	"github.com/stretchr/testify/require"
	"testing"
)

const GoWorkFixture = `
go 1.20

use (
	./core
	./fx
	./platform
	./x9
)
`

func TestParseString(t *testing.T) {
	mods, err := ParseString(GoWorkFixture)
	require.NoError(t, err)
	require.ElementsMatch(t, mods, []string{
		"core",
		"fx",
		"platform",
		"x9",
	})
}
