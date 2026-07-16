package webui

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAssetsEmbedsIndexHTML(t *testing.T) {
	index, err := Assets.ReadFile("build/index.html")
	require.NoError(t, err)
	require.NotEmpty(t, index)
}
