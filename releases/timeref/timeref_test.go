package timeref

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsValid(t *testing.T) {
	require.True(t, IsValid(refFormat))
}
