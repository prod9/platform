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
	version, mods, err := ParseString(GoWorkFixture)
	require.NoError(t, err)
	require.Equal(t, "1.20", version)
	require.ElementsMatch(t, mods, []string{
		"core",
		"fx",
		"platform",
		"x9",
	})
}
